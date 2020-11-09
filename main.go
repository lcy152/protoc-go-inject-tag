package main

import (
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var Debug = false

func main() {
	var workDir string
	workDir, _ = os.Getwd()
	println(workDir)
	if Debug {
		workDir = "D:/workspace/protoc-go-inject-tag-master/dd"
	}

	protoFileList, _ := GetAllProtoFiles("proto_file")
	protoHash := ProtoHash{}
	protoHash.HashMap = make(map[string]string)
	protoHash.PathMap = make(map[string]string)
	protoHash.NameMap = make(map[string]ProtoI)
	protoHash.VersionMap = make(map[string]int32)
	for _, dir := range protoFileList {
		file, err := os.Open(dir)
		if err != nil {
			log.Fatal("open md5 file error")
		}
		defer file.Close()
		h := md5.New()
		if _, err := io.Copy(h, file); err != nil {
			log.Fatal(err)
		}
		contentHash := h.Sum(nil)

		pathSep := string(os.PathSeparator)
		reg := regexp.MustCompile(`\\.*?proto`)
		modelName := strings.ReplaceAll(reg.FindString(dir), pathSep, "")
		modelNameList := strings.Split(modelName, ".")
		if len(modelNameList) > 0 {
			structName := HandleProtoPath(modelNameList[0])
			protoHash.HashMap[structName] = fmt.Sprintf("%x", contentHash)
			protoHash.PathMap[modelNameList[0]] = structName
		}
	}

	fileList, _ := GetAllFiles(workDir)
	file, err := os.Open("regular.json")
	if err != nil {
		log.Print("parse json error")
		return
	}
	defer func() {
		_ = file.Close()
	}()
	regular, _ := ioutil.ReadAll(file)
	var regularConfig RegularConfig
	err = json.Unmarshal(regular, &regularConfig)
	if err != nil {
		log.Print(err)
		return
	}
	typeMap := make(map[string]TypeConfig)
	for _, v := range regularConfig.TypeList {
		typeMap[v.Target] = TypeConfig{
			Target:          v.Target,
			Replace:         v.Replace,
			RemoveOmitempty: v.RemoveOmitempty,
			ImportList:      v.ImportList,
		}
	}
	tagMap := make(map[string]string)
	for _, v := range regularConfig.BsonTagList {
		tagMap[v.Target] = v.Replace
	}
	nameMap := make(map[string]string)
	for _, v := range regularConfig.NameList {
		nameMap[v.RepeatName] = v.ReName
	}
	config := Config{
		Type: typeMap,
		Tag:  tagMap,
		Name: nameMap,
	}

	var dslFileList []string
	for _, dir := range fileList {
		if dir[len(dir)-5:len(dir)] == "pb.go" {
			var xxxSkipSlice []string
			xxxSkipSlice = append(xxxSkipSlice, "bson")
			areas, err := removeSingleTypeOmitempty(dir, xxxSkipSlice, config)
			if err != nil {
				log.Fatal(err)
			}
			if err = writeFile1(dir, areas); err != nil {
				log.Fatal(err)
			}
			if strings.Index(dir, "dsl_base") != -1 {
				continue
			}
			var outputPath string
			if err, outputPath = CopyFileAndRename(dir); err != nil {
				log.Fatal(err)
			}
			dslFileList = append(dslFileList, outputPath)
		}
	}
	for _, dir := range dslFileList {
		if strings.Index(dir, "dsl_base") != -1 {
			BaseHandler(dir)
			continue
		}
		if err := deleteFunc(dir, protoHash); err != nil {
			log.Fatal(err)
		}
		importLog := false
		if err := CreateToProtoFunc(dir, config, protoHash, &importLog); err != nil {
			log.Fatal(err)
		}
		if err := CreateFromProtoFunc(dir, config); err != nil {
			log.Fatal(err)
		}
		var xxxSkipSlice []string
		xxxSkipSlice = append(xxxSkipSlice, "bson")
		areas, injectImport, err := parseFile1(dir, xxxSkipSlice, config)
		if importLog {
			injectImport = append(injectImport, "log")
		}
		if err != nil {
			log.Fatal(err)
		}
		if err = writeFile1(dir, areas); err != nil {
			log.Fatal(err)
		}
		if err := addImports(dir, injectImport, config); err != nil {
			log.Fatal(err)
		}
	}
}

func parseFile1(inputPath string, xxxSkip []string, config Config) (areas []textArea, injectImport []string, err error) {
	log.Printf("parsing file %q for inject tag comments", inputPath)
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, inputPath, nil, parser.ParseComments)
	if err != nil {
		return
	}

	for _, decl := range f.Decls {
		// check if is generic declaration
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		var typeSpec *ast.TypeSpec
		var valueSpec *ast.ValueSpec
		for _, spec := range genDecl.Specs {
			if ts, tsOK := spec.(*ast.TypeSpec); tsOK {
				typeSpec = ts
				break
			}
			if ts, tsOK := spec.(*ast.ValueSpec); tsOK {
				valueSpec = ts
				break
			}
		}

		if valueSpec != nil {
			for _, u := range valueSpec.Names {
				if config.Name[u.Name] != "" {
					area := textArea{
						Start:       int(valueSpec.Pos()),
						End:         int(valueSpec.End()),
						CurrentTag:  "",
						InjectTag:   "",
						CurrentType: "",
						InjectType:  "",
						CurrentName: u.Name,
						InjectName:  config.Name[u.Name],
					}
					areas = append(areas, area)
				}
			}
		}

		// skip if can't get type spec
		if typeSpec == nil {
			continue
		}

		// not a struct, skip
		structDecl, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}

		for _, field := range structDecl.Fields.List {
			// skip if field has no doc
			if len(field.Names) > 0 {
				//name := field.Names[0].Name
				if len(xxxSkip) > 0 { //&& strings.HasPrefix(name, "XXX")
					builder := strings.Builder{}
					currentTag := field.Tag.Value
					// _, ok := field.Type.(*ast.Ident)
					// if ok {
					currentTag = strings.Replace(currentTag, ",omitempty", "", -1)
					// }
					currentType := ""
					InjectType := ""
					var fieldExpr *ast.StarExpr
					if u, ok := field.Type.(*ast.StarExpr); ok {
						fieldExpr = u
					} else if v, ok := field.Type.(*ast.ArrayType); ok {
						if u, ok := v.Elt.(*ast.StarExpr); ok {
							fieldExpr = u
						}
					}
					if fieldExpr != nil {
						if w, ok := fieldExpr.X.(*ast.SelectorExpr); ok {
							if w1, ok := w.X.(*ast.Ident); ok {
								ptrName := "*" + w1.Name + "." + w.Sel.Name
								if config.Type[ptrName].Replace != "" {
									currentType = ptrName
									InjectType = config.Type[ptrName].Replace
									injectImport = append(injectImport, config.Type[ptrName].ImportList...)
									if config.Type[ptrName].RemoveOmitempty {
										currentTag = strings.Replace(currentTag, ",omitempty", "", -1)
									}
								}
							}
						}
					}

					TagValue := currentTag[strings.Index(currentTag, "json:\"")+6 : len(currentTag)-2]
					if config.Tag[TagValue] != "" {
						TagValue = config.Tag[TagValue]
					}
					for i, skip := range xxxSkip {
						builder.WriteString(fmt.Sprintf("%s:\"%s\"", skip, TagValue))
						if i > 0 {
							builder.WriteString(",")
						}
					}
					area := textArea{
						Start:       int(field.Pos()),
						End:         int(field.End()),
						CurrentTag:  currentTag[1 : len(currentTag)-1],
						InjectTag:   builder.String(),
						CurrentType: currentType,
						InjectType:  InjectType,
					}
					areas = append(areas, area)
				}
			}
			if field.Doc == nil {
				continue
			}
			for _, comment := range field.Doc.List {
				tag := TagFromComment(comment.Text)
				if tag == "" {
					continue
				}
				currentTag := field.Tag.Value
				area := textArea{
					Start:      int(field.Pos()),
					End:        int(field.End()),
					CurrentTag: currentTag[1 : len(currentTag)-1],
					InjectTag:  tag,
				}
				areas = append(areas, area)
			}
		}

	}

	log.Printf("parsed file %q, number of fields to inject custom tags: %d", inputPath, len(areas))
	return
}

func removeSingleTypeOmitempty(inputPath string, xxxSkip []string, config Config) (areas []textArea, err error) {
	log.Printf("parsing file %q for inject tag comments", inputPath)
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, inputPath, nil, parser.ParseComments)
	if err != nil {
		return
	}
	for _, decl := range f.Decls {
		// check if is generic declaration
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		var typeSpec *ast.TypeSpec
		for _, spec := range genDecl.Specs {
			if ts, tsOK := spec.(*ast.TypeSpec); tsOK {
				typeSpec = ts
				break
			}
		}
		// skip if can't get type spec
		if typeSpec == nil {
			continue
		}
		// not a struct, skip
		structDecl, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}
		for _, field := range structDecl.Fields.List {
			// skip if field has no doc
			if len(field.Names) > 0 {
				//name := field.Names[0].Name
				if len(xxxSkip) > 0 { //&& strings.HasPrefix(name, "XXX")
					builder := strings.Builder{}
					currentTag := field.Tag.Value
					_, ok := field.Type.(*ast.Ident)
					if ok {
						currentTag = strings.Replace(currentTag, ",omitempty", "", -1)
					}
					TagValue := currentTag[strings.Index(currentTag, "json:\"")+6 : len(currentTag)-2]
					if config.Tag[TagValue] != "" {
						TagValue = config.Tag[TagValue]
					}
					for i, skip := range xxxSkip {
						builder.WriteString(fmt.Sprintf("%s:\"%s\"", skip, TagValue))
						if i > 0 {
							builder.WriteString(",")
						}
					}
					area := textArea{
						Start: int(field.Pos()),
						End:   int(field.End()),
						// CurrentTag: currentTag[1 : len(currentTag)-1],
						// InjectTag:  builder.String(),
						CurrentTag: field.Tag.Value,
						InjectTag:  currentTag[1 : len(currentTag)-1],
					}
					areas = append(areas, area)
				}
			}
			if field.Doc == nil {
				continue
			}
			for _, comment := range field.Doc.List {
				tag := TagFromComment(comment.Text)
				if tag == "" {
					continue
				}
				currentTag := field.Tag.Value
				area := textArea{
					Start:      int(field.Pos()),
					End:        int(field.End()),
					CurrentTag: currentTag[1 : len(currentTag)-1],
					InjectTag:  tag,
				}
				areas = append(areas, area)
			}
		}
	}
	return
}

func parseFunc(inputPath string, config Config) (err error) {
	log.Printf("parsing file %q for inject tag comments", inputPath)
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, inputPath, nil, parser.ParseComments)
	if err != nil {
		return
	}
	for _, decl := range f.Decls {
		if funDecl, ok := decl.(*ast.FuncDecl); ok {
			ParseTimeStamp(funDecl, config)
			// ParseVec(funDecl, config)
		}
	}
	var output []byte
	buffer := bytes.NewBuffer(output)
	err = format.Node(buffer, fSet, f)
	if err != nil {
		log.Fatal(err)
	}
	// 输出Go代码
	if err = ioutil.WriteFile(inputPath, []byte(buffer.String()), 0644); err != nil {
		return
	}

	return
}

func contentReplace(inputPath string, config Config) (err error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return
	}

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	if err = f.Close(); err != nil {
		return
	}
	for _, t := range config.Type {
		count := strings.Count(t.Target, "*")
		expReg := t.Target
		if count > 0 {
			expReg = "\\" + expReg
		}
		contents = regexp.MustCompile(expReg).ReplaceAll(contents, []byte(t.Replace))
	}
	// 输出Go代码
	if err = ioutil.WriteFile(inputPath, contents, 0644); err != nil {
		return
	}

	return
}

func deleteFunc(inputPath string, protoHash ProtoHash) (err error) {
	log.Printf("parsing file %q for inject tag comments", inputPath)
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, inputPath, nil, parser.ParseComments)
	if err != nil {
		return
	}
	copyF := f.Decls[0:0]
	f.Comments = []*ast.CommentGroup{}
	var selfDecls []ast.Decl
	for _, decl := range f.Decls {
		d, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		var value, hash, name string
		for _, v := range d.Specs {
			if _, ok := v.(*ast.TypeSpec); ok {
				copyF = append(copyF, decl)
				break
			} else if _, ok := v.(*ast.ImportSpec); ok {
				copyF = append(copyF, decl)
				break
			} else if v2, ok := v.(*ast.ValueSpec); ok {
				if strings.Index(v2.Names[0].Name, "dsl_type_version") != -1 {
					name_long := ""
					w_out := ""
					for k, w := range protoHash.PathMap {
						if strings.Index(inputPath, k) != -1 {
							if len(name_long) < len(k) {
								name_long = k
								w_out = w
							}
						}
					}
					if name_long != "" {
						if v3, ok := v2.Values[0].(*ast.BasicLit); ok {
							ii, err := strconv.ParseInt(v3.Value, 10, 32)
							if err == nil {
								protoHash.VersionMap[w_out] = int32(ii)
								value = v3.Value
								hash = protoHash.HashMap[w_out]
								name = w_out
							}
						}
					}
				}
			}
		}
		fmt.Println(value, hash)
		if value != "" {
			VersionSpec := &ast.ValueSpec{
				Names:  []*ast.Ident{&ast.Ident{Name: name + DslTypeVersion}},
				Values: []ast.Expr{&ast.BasicLit{Kind: token.INT, Value: value}},
			}
			HashSpecl := &ast.ValueSpec{
				Names:  []*ast.Ident{&ast.Ident{Name: name + DslTypeHash}},
				Values: []ast.Expr{&ast.BasicLit{Kind: token.STRING, Value: "\"" + hash + "\""}},
			}
			selfDecls = append(selfDecls, &ast.GenDecl{
				Specs: []ast.Spec{VersionSpec, HashSpecl},
				Tok:   token.CONST,
			})
			protoHash.NameMap[name] = ProtoI{HashName: DslTypeHash, VersionName: DslTypeVersion}
		}
	}
	copyF = append(copyF, selfDecls...)
	f.Decls = copyF
	var output []byte
	buffer := bytes.NewBuffer(output)
	err = format.Node(buffer, fSet, f)
	if err != nil {
		log.Fatal(err)
	}
	// 输出Go代码
	if err = ioutil.WriteFile(inputPath, []byte(buffer.String()), 0644); err != nil {
		return
	}

	return
}

func addImports(inputPath string, injectImport []string, config Config) (err error) {

	log.Printf("parsing file %q for inject tag comments", inputPath)
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, inputPath, nil, parser.ParseComments)
	if err != nil {
		return
	}
	for _, decl := range f.Decls {
		// check if is generic declaration
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		var valueList []ast.Spec
		if genDecl.Tok == token.IMPORT {
			for _, value := range injectImport {
				hasImported := false
				for _, v := range genDecl.Specs {
					importSpec := v.(*ast.ImportSpec)
					// 如果已经包含"context"
					if strings.ReplaceAll(strconv.Quote(importSpec.Path.Value), "\"", "") == strings.ReplaceAll(strconv.Quote(value), "\"", "") {
						hasImported = true
					}
				}
				// 如果没有import context，则import
				if !hasImported {
					valueList = append(valueList, &ast.ImportSpec{
						Path: &ast.BasicLit{
							Kind:  token.STRING,
							Value: strconv.Quote(value),
						},
					})
				}
			}
			genDecl.Specs = valueList
		}
	}

	var output []byte
	buffer := bytes.NewBuffer(output)
	err = format.Node(buffer, fSet, f)
	if err != nil {
		log.Fatal(err)
	}
	// 输出Go代码
	if err = ioutil.WriteFile(inputPath, []byte(buffer.String()), 0644); err != nil {
		return
	}

	return
}

func writeFile1(inputPath string, areas []textArea) (err error) {
	f, err := os.Open(inputPath)
	if err != nil {
		return
	}

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}

	if err = f.Close(); err != nil {
		return
	}

	for i := range areas {
		area := areas[len(areas)-i-1]
		log.Printf("inject custom tag %q to expression %q", area.InjectTag, string(contents[area.Start-1:area.End-1]))
		contents = InjectTag(contents, area)
	}
	if err = ioutil.WriteFile(inputPath, contents, 0644); err != nil {
		return
	}

	if len(areas) > 0 {
		log.Printf("file %q is injected with custom tags", inputPath)
	}
	return
}

func CopyFileAndRename(inputPath string) (err error, outputPath string) {
	f, err := os.Open(inputPath)
	if err != nil {
		return
	}
	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return
	}
	if err = f.Close(); err != nil {
		return
	}
	outputPath = strings.ReplaceAll(inputPath, "pb.go", "dsl.go")
	if err = ioutil.WriteFile(outputPath, contents, 0644); err != nil {
		return
	}
	return
}

func GetAllFiles(dirPth string) (files []string, err error) {
	var dirs []string
	dir, err := ioutil.ReadDir(dirPth)
	if err != nil {
		return nil, err
	}

	PthSep := string(os.PathSeparator)

	for _, fi := range dir {
		if fi.IsDir() {
			dirs = append(dirs, dirPth+PthSep+fi.Name())
			GetAllFiles(dirPth + PthSep + fi.Name())
		} else {
			ok := strings.HasSuffix(fi.Name(), ".go")
			if ok {
				files = append(files, dirPth+PthSep+fi.Name())
			}
		}
	}

	for _, table := range dirs {
		temp, _ := GetAllFiles(table)
		for _, temp1 := range temp {
			files = append(files, temp1)
		}
	}

	return files, nil
}

func GetAllProtoFiles(dirPth string) (files []string, err error) {
	var dirs []string
	dir, err := ioutil.ReadDir(dirPth)
	if err != nil {
		return nil, err
	}

	PthSep := string(os.PathSeparator)

	for _, fi := range dir {
		if fi.IsDir() {
			dirs = append(dirs, dirPth+PthSep+fi.Name())
			GetAllProtoFiles(dirPth + PthSep + fi.Name())
		} else {
			ok := strings.HasSuffix(fi.Name(), ".proto")
			if ok {
				files = append(files, dirPth+PthSep+fi.Name())
			}
		}
	}

	for _, table := range dirs {
		temp, _ := GetAllProtoFiles(table)
		for _, temp1 := range temp {
			files = append(files, temp1)
		}
	}

	return files, nil
}

func HandleProtoPath(dirPth string) string {
	if dirPth == "plan2" {
		dirPth = "plan"
	}
	charList := strings.Split(dirPth, "")
	retList := []string{}
	toUp := true
	for _, v := range charList {
		if toUp {
			retList = append(retList, strings.ToUpper(v))
			toUp = false
			continue
		}
		if v == "_" {
			toUp = true
			continue
		}
		retList = append(retList, v)
	}
	return strings.Join(retList, "")
}

func BaseHandler(inputPath string) {
	log.Printf("parsing file %q for inject tag comments", inputPath)
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, inputPath, nil, parser.ParseComments)
	if err != nil {
		return
	}
	copyF := f.Decls[0:0]
	for _, decl := range f.Decls {
		_, ok := decl.(*ast.FuncDecl)
		if !ok {
			copyF = append(copyF, decl)
			continue
		}
	}
	f.Decls = copyF
	var output []byte
	buffer := bytes.NewBuffer(output)
	err = format.Node(buffer, fSet, f)
	if err != nil {
		log.Fatal(err)
	}
	// 输出Go代码
	if err = ioutil.WriteFile(inputPath, []byte(buffer.String()), 0644); err != nil {
		return
	}

	return
}
