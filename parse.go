package main

import (
	"fmt"
	"go/ast"
	"regexp"
	"strings"
)

func TagFromComment(comment string) (tag string) {
	match := rComment.FindStringSubmatch(comment)
	if len(match) == 2 {
		tag = match[1]
	}
	return
}

type tagItem struct {
	key   string
	value string
}

type tagItems []tagItem

func (ti tagItems) format() string {
	tags := []string{}
	for _, item := range ti {
		tags = append(tags, fmt.Sprintf(`%s:%s`, item.key, item.value))
	}
	return strings.Join(tags, " ")
}

func (ti tagItems) override(nti tagItems) tagItems {
	overrided := []tagItem{}
	for i := range ti {
		var dup = -1
		for j := range nti {
			if ti[i].key == nti[j].key {
				dup = j
				break
			}
		}
		if dup == -1 {
			overrided = append(overrided, ti[i])
		} else {
			overrided = append(overrided, nti[dup])
			nti = append(nti[:dup], nti[dup+1:]...)
		}
	}
	return append(overrided, nti...)
}

func newTagItems(tag string) tagItems {
	items := []tagItem{}
	splitted := rTags.FindAllString(tag, -1)

	for _, t := range splitted {
		sepPos := strings.Index(t, ":")
		items = append(items, tagItem{
			key:   t[:sepPos],
			value: t[sepPos+1:],
		})
	}
	return items
}

func InjectTag(contents []byte, area textArea) (injected []byte) {
	expr := make([]byte, area.End-area.Start)
	copy(expr, contents[area.Start-1:area.End-1])
	cti := newTagItems(area.CurrentTag)
	iti := newTagItems(area.InjectTag)
	ti := cti.override(iti)
	expr = rInject.ReplaceAll(expr, []byte(fmt.Sprintf("`%s`", ti.format())))
	if len(area.InjectName) > 0 {
		expReg := area.CurrentName
		expr = regexp.MustCompile(expReg).ReplaceAll(expr, []byte(area.InjectName))
	}
	if len(area.InjectType) > 0 {
		count := strings.Count(area.CurrentType, "*")
		expReg := area.CurrentType
		if count > 0 {
			expReg = "\\" + area.CurrentType
		}
		expr = regexp.MustCompile(expReg).ReplaceAll(expr, []byte(area.InjectType))
	}
	print(fmt.Sprintf("`%s`", ti.format()))
	injected = append(injected, contents[:area.Start-1]...)
	injected = append(injected, expr...)
	injected = append(injected, contents[area.End-1:]...)
	return
}

func ParseTimeStamp(funDecl *ast.FuncDecl, config Config) {
	reList := funDecl.Type.Results
	change := false
	if reList != nil {
		for _, re2 := range reList.List {
			if typeName, ok := re2.Type.(*ast.StarExpr); ok {
				if typeN := typeName.X; ok {
					if typeN2, ok := typeN.(*ast.Ident); ok {
						if typeN2.Name == "TimeStamp" {
							// typeN2.Name = "primitive.DateTime"
							change = true
							temp := ast.SelectorExpr{}
							temp1 := ast.Ident{}
							temp2 := ast.Ident{}
							temp1.Name = "primitive"
							temp2.Name = "DateTime"
							temp.X = &temp1
							temp.Sel = &temp2
							re2.Type = &temp
						}
					}
				}
			}
		}
	}
	bodyList := funDecl.Body
	if bodyList != nil {
		for _, re2 := range bodyList.List {
			if typeName, ok := re2.(*ast.ReturnStmt); ok {
				for _, ret1 := range typeName.Results {
					if change {
						ret1.(*ast.Ident).Name = "0"
					}
					if ret2, ok := ret1.(*ast.Ident); ok {
						for k, v := range config.Name {
							if k == ret2.Name {
								ret1.(*ast.Ident).Name = v
							}
						}
					}
				}
			}
		}
	}
}

func ParseVec(funDecl *ast.FuncDecl, config Config) {
	reList := funDecl.Type.Results
	if reList != nil {
		for _, re2 := range reList.List {
			if typeName, ok := re2.Type.(*ast.StarExpr); ok {
				if typeN := typeName.X; ok {
					if typeN2 := typeN.(*ast.Ident); ok {
						ptrName := "*" + typeN2.Name
						if config.Type[ptrName].Replace != "" && strings.Index(config.Type[ptrName].Replace, "[") != -1 {
							temp := ast.ArrayType{}
							temp1 := ast.Ident{}
							temp1.Name = strings.Replace(strings.Replace(config.Type[ptrName].Replace, "[", "", 1), "]", "", 1)
							temp.Elt = &temp1
							re2.Type = &temp
						}
					}
				}
			}
		}
	}
}
