package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
)

const (
	stInRefName    = "i"
	stOutRefName   = "o"
	DslTypeHash    = "DslFileHash"
	DslTypeVersion = "DslTypeVersion"
)

func CreateToProtoFunc(inputPath string, config Config, protoHash ProtoHash, importLog *bool) (err error) {
	var structNameList []string
	type textArea struct {
		CurrentName string
		InjectName  string
	}
	fSet := token.NewFileSet()
	f, err := parser.ParseFile(fSet, inputPath, nil, parser.ParseComments)
	if err != nil {
		return
	}
	for _, decl := range f.Decls {
		d, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, v := range d.Specs {
			var typeSpec *ast.TypeSpec
			if typeSpec, ok = v.(*ast.TypeSpec); !ok {
				continue
			}
			structOriginName := typeSpec.Name.Name
			typeSpec.Name.Name = GetDslName(typeSpec.Name.Name)
			structNameList = append(structNameList, structOriginName)

			structDecl, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			ToProtoFunc := &ast.FuncDecl{}
			funcRecvList := &ast.FieldList{}
			var funcRecvNameList []*ast.Ident
			funcRecvNameList = append(funcRecvNameList, &ast.Ident{Name: stInRefName})
			funcRecvType := &ast.StarExpr{X: &ast.Ident{Name: typeSpec.Name.Name}}
			funcRecvList.List = append(funcRecvList.List, &ast.Field{Names: funcRecvNameList, Type: funcRecvType})
			ToProtoFunc.Recv = funcRecvList
			ToProtoFunc.Name = &ast.Ident{Name: "ToProto"}
			funcType := &ast.FuncType{}

			funcResults := &ast.FieldList{}
			resultType := &ast.StarExpr{X: &ast.Ident{Name: structOriginName}}
			funcResults.List = append(funcResults.List, &ast.Field{Type: resultType})
			funcType.Results = funcResults
			ToProtoFunc.Type = funcType

			var bodyList []ast.Stmt

			var dimVarLhs []ast.Expr
			dimVarLhs = append(dimVarLhs, &ast.Ident{Name: stOutRefName})
			var dimVarRhs []ast.Expr
			dimVarRhs_1 := &ast.UnaryExpr{}
			dimVarRhs_1.Op = token.AND
			dimVarRhs_1_X := &ast.CompositeLit{}
			dimVarRhs_1_X.Type = &ast.Ident{Name: structOriginName}
			dimVarRhs_1.X = dimVarRhs_1_X
			dimVarRhs = append(dimVarRhs, dimVarRhs_1)
			dimReturnObj := &ast.AssignStmt{
				Lhs: dimVarLhs,
				Tok: token.DEFINE,
				Rhs: dimVarRhs,
			}
			bodyList = append(bodyList, dimReturnObj)

			var copyStructList []*ast.Field
			for _, field := range structDecl.Fields.List {
				if len(field.Names) > 0 {
					if len(field.Names[0].Name) > 3 && field.Names[0].Name[0:4] == "XXX_" {
						continue
					}
					copyStructList = append(copyStructList, field)
				}
			}
			structDecl.Fields.List = copyStructList

			for _, field := range structDecl.Fields.List {
				if len(field.Names) > 0 {
					if len(field.Names[0].Name) > 3 && field.Names[0].Name[0:4] == "XXX_" {
						continue
					}
					SetSingleValue := func(field *ast.Field) {
						leftSelector := &ast.SelectorExpr{}
						rightSelector := &ast.SelectorExpr{}
						leftSelector.X = &ast.Ident{Name: stOutRefName}
						rightSelector.X = &ast.Ident{Name: stInRefName}
						leftSelector.Sel = &ast.Ident{Name: field.Names[0].Name}
						rightSelector.Sel = &ast.Ident{Name: field.Names[0].Name}
						var lhs []ast.Expr
						var rhs []ast.Expr
						lhs = append(lhs, leftSelector)
						rhs = append(rhs, rightSelector)
						bodyItem := &ast.AssignStmt{
							Lhs: lhs,
							Tok: token.ASSIGN,
							Rhs: rhs,
						}
						bodyList = append(bodyList, bodyItem)
					}
					if _, ok := field.Type.(*ast.Ident); ok {
						SetSingleValue(field)
					} else if arr, ok := field.Type.(*ast.ArrayType); ok {
						if SingleArrayType(arr) {
							SetSingleValue(field)
						} else {
							isBaseType := false
							if t, ok := field.Type.(*ast.ArrayType); ok {
								if t1, ok := t.Elt.(*ast.StarExpr); ok {
									if t2, ok := t1.X.(*ast.SelectorExpr); ok {
										if t3, ok := t2.X.(*ast.Ident); ok {
											if "DslBase" == t3.Name {
												isBaseType = true
												bodyList = ListBaseTypeTrandformToProto(field, t2.Sel.Name, bodyList)
											}
										}
									}
								}
							}
							if !isBaseType {
								leftTemp := &ast.Ident{Name: "temp"}
								var lhs1 []ast.Expr
								lhs1 = append(lhs1, leftTemp)
								var rhs1 []ast.Expr
								inSelector := &ast.SelectorExpr{}
								inSelector.X = &ast.Ident{Name: "v"}
								inSelector.Sel = &ast.Ident{Name: "ToProto"}
								rightSelector := &ast.CallExpr{}
								rightSelector.Fun = inSelector
								rhs1 = append(rhs1, rightSelector)
								body1 := &ast.AssignStmt{Lhs: lhs1, Rhs: rhs1, Tok: token.DEFINE}
								leftSelector2 := &ast.SelectorExpr{}
								leftSelector2.X = &ast.Ident{Name: stOutRefName}
								leftSelector2.Sel = &ast.Ident{Name: field.Names[0].Name}
								rightSelector2 := &ast.CallExpr{}
								rightSelector2.Fun = &ast.Ident{Name: "append"}
								var args []ast.Expr
								selector1 := &ast.SelectorExpr{
									X:   &ast.Ident{Name: stOutRefName},
									Sel: &ast.Ident{Name: field.Names[0].Name},
								}
								selector2 := &ast.Ident{Name: "temp"}
								args = append(args, selector1, selector2)
								rightSelector2.Args = args
								var lhs2 []ast.Expr
								var rhs2 []ast.Expr
								lhs2 = append(lhs2, leftSelector2)
								rhs2 = append(rhs2, rightSelector2)
								body2 := &ast.AssignStmt{
									Lhs: lhs2,
									Tok: token.ASSIGN,
									Rhs: rhs2,
								}
								var complexAss []ast.Stmt
								complexAss = append(complexAss, body1, body2)
								rangeBody := &ast.BlockStmt{}
								rangeBody.List = complexAss
								rangeItem := &ast.RangeStmt{}
								rangeItem.Body = rangeBody
								rangeItem.Key = &ast.Ident{Name: "_"}
								rangeItem.Value = &ast.Ident{Name: "v"}
								rangeItem.Tok = token.DEFINE
								rangeItem_X := &ast.SelectorExpr{}
								rangeItem_X.X = &ast.Ident{Name: stInRefName}
								rangeItem_X.Sel = &ast.Ident{Name: field.Names[0].Name}
								rangeItem.X = rangeItem_X
								bodyList = append(bodyList, rangeItem)
								if arrIn, ok := arr.Elt.(*ast.StarExpr); ok {
									if arrInIn, ok := arrIn.X.(*ast.Ident); ok {
										arrInIn.Name = GetDslName(arrInIn.Name)
									}
								}

							}
						}
					} else {
						isBaseType := false
						if t, ok := field.Type.(*ast.StarExpr); ok {
							if t1, ok := t.X.(*ast.SelectorExpr); ok {
								if t2, ok := t1.X.(*ast.Ident); ok {
									if "DslBase" == t2.Name {
										isBaseType = true
										bodyList = BaseTypeTrandformToProto(field, t1.Sel.Name, bodyList)
										continue
									}
								}
							}
						}
						if !isBaseType {
							leftSelector2 := &ast.SelectorExpr{}
							leftSelector2.X = &ast.Ident{Name: stOutRefName}
							leftSelector2.Sel = &ast.Ident{Name: field.Names[0].Name}
							rightSelector2 := &ast.SelectorExpr{}
							rightSelector2.X = &ast.Ident{Name: stInRefName}
							rightSelector2.Sel = &ast.Ident{Name: field.Names[0].Name}
							var lhs2 []ast.Expr
							lhs2 = append(lhs2, leftSelector2)
							inSelector := &ast.SelectorExpr{}
							inSelector.X = rightSelector2
							inSelector.Sel = &ast.Ident{Name: "ToProto"}
							var rhs []ast.Expr
							rightSelector := &ast.CallExpr{}
							rightSelector.Fun = inSelector
							rhs = append(rhs, rightSelector)
							body2 := &ast.AssignStmt{
								Lhs: lhs2,
								Tok: token.ASSIGN,
								Rhs: rhs,
							}
							IfStruct := &ast.IfStmt{
								Cond: &ast.BinaryExpr{
									X: &ast.SelectorExpr{
										X:   &ast.Ident{Name: stInRefName},
										Sel: &ast.Ident{Name: field.Names[0].Name},
									},
									Op: token.NEQ,
									Y:  &ast.BasicLit{Value: "nil"},
								},
								Body: &ast.BlockStmt{
									List: []ast.Stmt{
										body2,
									},
								},
							}
							bodyList = append(bodyList, IfStruct)
							if arrIn, ok := field.Type.(*ast.StarExpr); ok {
								if arrInIn, ok := arrIn.X.(*ast.Ident); ok {
									arrInIn.Name = GetDslName(arrInIn.Name)
								}
							}
						}
					}
				}
			}
			resultItem := &ast.Ident{Name: stOutRefName}
			returnStmt := &ast.ReturnStmt{}
			var bodyResult []ast.Expr
			bodyResult = append(bodyResult, resultItem)
			returnStmt.Results = bodyResult
			if protoHash.VersionMap[structOriginName] != 0 {
				*importLog = true
				protoHandlerFunc := &ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.SelectorExpr{
							X:   &ast.Ident{Name: stInRefName},
							Sel: &ast.Ident{Name: DslTypeHash},
						},
						Op: token.NEQ,
						Y:  &ast.Ident{Name: structOriginName + DslTypeHash},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.IfStmt{
								Cond: &ast.BinaryExpr{
									X: &ast.BinaryExpr{
										X: &ast.SelectorExpr{
											X:   &ast.Ident{Name: stInRefName},
											Sel: &ast.Ident{Name: DslTypeVersion},
										},
										Op: token.QUO,
										Y:  &ast.BasicLit{Kind: token.INT, Value: "1000"},
									},
									Op: token.NEQ,
									Y: &ast.BinaryExpr{
										X:  &ast.Ident{Name: structOriginName + DslTypeVersion},
										Op: token.QUO,
										Y:  &ast.BasicLit{Kind: token.INT, Value: "1000"},
									},
								},
								Body: &ast.BlockStmt{
									List: []ast.Stmt{
										&ast.ExprStmt{
											X: &ast.CallExpr{
												// Fun: &ast.Ident{Name: "panic"},
												Fun: &ast.SelectorExpr{
													X:   &ast.Ident{Name: "log"},
													Sel: &ast.Ident{Name: "Println"},
												},
												Args: []ast.Expr{
													&ast.BasicLit{Kind: token.STRING, Value: "\"proto version error\""},
												},
											},
										},
									},
								},
							},
						},
					},
				}
				bodyList = append(bodyList, protoHandlerFunc)
				hashTag := "`protobuf:\"bytes,2,opt,name=dsl_file_hash,proto3\" json:\"dsl_file_hash\" bson:\"dsl_file_hash,omitempty\"`"
				versionTag := "`protobuf:\"bytes,2,opt,name=dsl_type_version,proto3\" json:\"dsl_type_version\" bson:\"dsl_type_version,omitempty\"`"
				protoTypeList := []*ast.Field{
					&ast.Field{
						Names: []*ast.Ident{
							&ast.Ident{Name: protoHash.NameMap[structOriginName].HashName},
						},
						Type: &ast.Ident{Name: "string"},
						Tag:  &ast.BasicLit{Kind: token.STRING, Value: hashTag},
					},
					&ast.Field{
						Names: []*ast.Ident{
							&ast.Ident{Name: protoHash.NameMap[structOriginName].VersionName},
						},
						Type: &ast.Ident{Name: "int"},
						Tag:  &ast.BasicLit{Kind: token.INT, Value: versionTag},
					},
				}
				structDecl.Fields.List = append(structDecl.Fields.List, protoTypeList...)
			}
			bodyList = append(bodyList, returnStmt)
			funcBody := &ast.BlockStmt{}
			funcBody.List = bodyList
			ToProtoFunc.Body = funcBody
			f.Decls = append(f.Decls, ToProtoFunc)
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

func BaseTypeTrandformToProto(in *ast.Field, tp string, bodyList []ast.Stmt) []ast.Stmt {
	switch tp {
	case "TimeStamp":
		leftSelector1 := &ast.SelectorExpr{}
		leftSelector1.X = &ast.Ident{Name: stOutRefName}
		leftSelector1.Sel = &ast.Ident{Name: in.Names[0].Name}
		var lhs2 []ast.Expr
		lhs2 = append(lhs2, leftSelector1)
		selector2 := &ast.SelectorExpr{}
		selector2.X = &ast.Ident{Name: stInRefName}
		selector2.Sel = &ast.Ident{Name: in.Names[0].Name}
		rightSelector2 := &ast.CallExpr{}
		rightSelector2.Fun = &ast.Ident{Name: "int64"}
		var args []ast.Expr
		args = append(args, selector2)
		rightSelector2.Args = args
		dimVarRhs_1 := &ast.KeyValueExpr{}
		dimVarRhs_1.Key = &ast.Ident{Name: "Time"}
		dimVarRhs_1.Value = rightSelector2
		var rhs1 []ast.Expr
		rhs1 = append(rhs1, dimVarRhs_1)
		unaryExpr := &ast.UnaryExpr{
			Op: token.AND,
			X: &ast.CompositeLit{
				Type: &ast.SelectorExpr{
					X:   &ast.Ident{Name: "DslBase"},
					Sel: &ast.Ident{Name: tp},
				},
				Elts: rhs1,
			},
		}
		var rhs2 []ast.Expr
		rhs2 = append(rhs2, unaryExpr)
		body2 := &ast.AssignStmt{
			Lhs: lhs2,
			Tok: token.ASSIGN,
			Rhs: rhs2,
		}
		bodyList = append(bodyList, body2)
	case "Ivec2", "Fvec2", "Dvec2":
		ParamMap := []string{"X", "Y"}
		t, ifStruct := GetVecUtilToProto(ParamMap, in.Names[0].Name, tp, "2")
		bodyList = append(bodyList, t, ifStruct)
	case "Ivec3", "Fvec3", "Dvec3":
		ParamMap := []string{"X", "Y", "Z"}
		t, ifStruct := GetVecUtilToProto(ParamMap, in.Names[0].Name, tp, "3")
		bodyList = append(bodyList, t, ifStruct)
	case "Dmat3", "Dmat4":
		leftSelector1 := &ast.SelectorExpr{}
		leftSelector1.X = &ast.Ident{Name: stOutRefName}
		leftSelector1.Sel = &ast.Ident{Name: in.Names[0].Name}
		var lhs2 []ast.Expr
		lhs2 = append(lhs2, leftSelector1)
		selector2 := &ast.SelectorExpr{}
		selector2.X = &ast.Ident{Name: stInRefName}
		selector2.Sel = &ast.Ident{Name: in.Names[0].Name}
		dimVarRhs_1 := &ast.KeyValueExpr{}
		dimVarRhs_1.Key = &ast.Ident{Name: "Data"}
		dimVarRhs_1.Value = selector2
		var rhs1 []ast.Expr
		rhs1 = append(rhs1, dimVarRhs_1)
		unaryExpr := &ast.UnaryExpr{
			Op: token.AND,
			X: &ast.CompositeLit{
				Type: &ast.SelectorExpr{
					X:   &ast.Ident{Name: "DslBase"},
					Sel: &ast.Ident{Name: tp},
				},
				Elts: rhs1,
			},
		}
		var rhs2 []ast.Expr
		rhs2 = append(rhs2, unaryExpr)
		body2 := &ast.AssignStmt{
			Lhs: lhs2,
			Tok: token.ASSIGN,
			Rhs: rhs2,
		}
		bodyList = append(bodyList, body2)
	case "Path2F":
		ParamMap := []string{"X", "Y"}
		body2, ifStruct := GetPathUtilToProto(ParamMap, in.Names[0].Name, tp, "Fvec2", "2")
		bodyList = append(bodyList, body2, ifStruct)
	case "Path3F":
		ParamMap := []string{"X", "Y", "Z"}
		body2, ifStruct := GetPathUtilToProto(ParamMap, in.Names[0].Name, tp, "Fvec3", "3")
		bodyList = append(bodyList, body2, ifStruct)
	case "Path2D":
		ParamMap := []string{"X", "Y"}
		body2, ifStruct := GetPathUtilToProto(ParamMap, in.Names[0].Name, tp, "Dvec2", "2")
		bodyList = append(bodyList, body2, ifStruct)
	case "Path3D":
		ParamMap := []string{"X", "Y", "Z"}
		body2, ifStruct := GetPathUtilToProto(ParamMap, in.Names[0].Name, tp, "Dvec3", "3")
		bodyList = append(bodyList, body2, ifStruct)
	}
	return bodyList
}

func GetVecUtilToProto(m []string, name, st, num string) (*ast.AssignStmt, *ast.IfStmt) {
	// dim
	leftSelector := &ast.SelectorExpr{}
	leftSelector.X = &ast.Ident{Name: stOutRefName}
	leftSelector.Sel = &ast.Ident{Name: name}
	var lhs []ast.Expr
	lhs = append(lhs, leftSelector)
	var rhs []ast.Expr
	unaryExpr := &ast.UnaryExpr{
		Op: token.AND,
		X: &ast.CompositeLit{
			Type: &ast.SelectorExpr{
				X:   &ast.Ident{Name: "DslBase"},
				Sel: &ast.Ident{Name: st},
			},
		},
	}
	rhs = append(rhs, unaryExpr)
	body2 := &ast.AssignStmt{
		Lhs: lhs,
		Tok: token.ASSIGN,
		Rhs: rhs,
	}
	// value
	var ParamList []ast.Stmt
	ParamMapTable := map[string]string{"X": "0", "Y": "1", "Z": "2"}
	for _, v := range m {
		temp := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X: &ast.SelectorExpr{
						X:   &ast.Ident{Name: stOutRefName},
						Sel: &ast.Ident{Name: name},
					},
					Sel: &ast.Ident{Name: v},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{
				&ast.IndexExpr{
					X: &ast.SelectorExpr{
						X:   &ast.Ident{Name: stInRefName},
						Sel: &ast.Ident{Name: name},
					},
					Index: &ast.BasicLit{Value: ParamMapTable[v], Kind: token.INT},
				},
			},
		}
		ParamList = append(ParamList, temp)
	}
	isIfStruct := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X: &ast.CallExpr{
				Fun: &ast.Ident{
					Name: "len",
				},
				Args: []ast.Expr{
					&ast.SelectorExpr{
						X: &ast.Ident{
							Name: stInRefName,
						},
						Sel: &ast.Ident{
							Name: name,
						},
					},
				},
			},
			Op: token.EQ,
			Y:  &ast.BasicLit{Value: num, Kind: token.INT},
		},
		Body: &ast.BlockStmt{
			List: ParamList,
		},
	}
	return body2, isIfStruct
}

func GetPathUtilToProto(m []string, name, st, st2, num string) (*ast.AssignStmt, *ast.RangeStmt) {
	leftSelector := &ast.SelectorExpr{}
	leftSelector.X = &ast.Ident{Name: stOutRefName}
	leftSelector.Sel = &ast.Ident{Name: name}
	var lhs []ast.Expr
	lhs = append(lhs, leftSelector)
	var rhs []ast.Expr
	unaryExpr := &ast.UnaryExpr{
		Op: token.AND,
		X: &ast.CompositeLit{Type: &ast.SelectorExpr{
			X:   &ast.Ident{Name: "DslBase"},
			Sel: &ast.Ident{Name: st},
		},
		},
	}
	rhs = append(rhs, unaryExpr)
	body2 := &ast.AssignStmt{
		Lhs: lhs,
		Tok: token.ASSIGN,
		Rhs: rhs,
	}
	var ParamList []ast.Stmt
	ParamMapTable := map[string]string{"X": "0", "Y": "1", "Z": "2"}
	for _, v := range m {
		temp := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: "temp"},
					Sel: &ast.Ident{Name: v},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{
				&ast.IndexExpr{
					X:     &ast.Ident{Name: "v"},
					Index: &ast.BasicLit{Value: ParamMapTable[v], Kind: token.INT},
				},
			},
		}
		ParamList = append(ParamList, temp)
	}
	rangeStmt := &ast.RangeStmt{
		Key:   &ast.Ident{Name: "_"},
		Value: &ast.Ident{Name: "v"},
		Tok:   token.DEFINE,
		X: &ast.SelectorExpr{
			X:   &ast.Ident{Name: stInRefName},
			Sel: &ast.Ident{Name: name},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: "temp"}},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{&ast.UnaryExpr{
						Op: token.AND,
						X: &ast.CompositeLit{Type: &ast.SelectorExpr{
							X:   &ast.Ident{Name: "DslBase"},
							Sel: &ast.Ident{Name: st2},
						}, Incomplete: false},
					}},
				},
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.CallExpr{
							Fun:  &ast.Ident{Name: "len"},
							Args: []ast.Expr{&ast.Ident{Name: "v"}},
						},
						Op: token.EQ,
						Y:  &ast.BasicLit{Value: num, Kind: token.INT},
					},
					Body: &ast.BlockStmt{
						List: ParamList,
					},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X: &ast.SelectorExpr{
								X:   &ast.Ident{Name: stOutRefName},
								Sel: &ast.Ident{Name: name},
							},
							Sel: &ast.Ident{Name: "Points"},
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun: &ast.Ident{Name: "append"},
						Args: []ast.Expr{
							&ast.SelectorExpr{
								X: &ast.SelectorExpr{
									X:   &ast.Ident{Name: stOutRefName},
									Sel: &ast.Ident{Name: name},
								},
								Sel: &ast.Ident{Name: "Points"},
							},
							&ast.Ident{Name: "temp"},
						},
					}},
				},
			},
		},
	}

	return body2, rangeStmt
}

func ListBaseTypeTrandformToProto(in *ast.Field, tp string, bodyList []ast.Stmt) []ast.Stmt {
	switch tp {
	case "Ivec2", "Fvec2", "Dvec2":
		ParamMap := map[string]string{"X": "0", "Y": "1"}
		ifStruct := ListGetVecUtilToProto(ParamMap, in.Names[0].Name, tp, "2")
		bodyList = append(bodyList, ifStruct)
	case "Ivec3", "Dvec3", "Fvec3":
		ParamMap := map[string]string{"X": "0", "Y": "1", "Z": "2"}
		ifStruct := ListGetVecUtilToProto(ParamMap, in.Names[0].Name, tp, "3")
		bodyList = append(bodyList, ifStruct)
	case "Dmat3", "Dmat4":
		rangeStmt := &ast.RangeStmt{
			Key:   &ast.Ident{Name: "_"},
			Value: &ast.Ident{Name: "v"},
			Tok:   token.DEFINE,
			X: &ast.SelectorExpr{
				X:   &ast.Ident{Name: stInRefName},
				Sel: &ast.Ident{Name: in.Names[0].Name},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.AssignStmt{
						Lhs: []ast.Expr{&ast.Ident{Name: "temp"}},
						Tok: token.DEFINE,
						Rhs: []ast.Expr{&ast.UnaryExpr{
							Op: token.AND,
							X: &ast.CompositeLit{Type: &ast.SelectorExpr{
								X:   &ast.Ident{Name: "DslBase"},
								Sel: &ast.Ident{Name: tp},
							}, Incomplete: false},
						}},
					},
					&ast.AssignStmt{
						Lhs: []ast.Expr{&ast.SelectorExpr{
							X:   &ast.Ident{Name: "temp"},
							Sel: &ast.Ident{Name: "Data"},
						}},
						Tok: token.DEFINE,
						Rhs: []ast.Expr{&ast.Ident{Name: "v"}},
					},
					&ast.AssignStmt{
						Lhs: []ast.Expr{
							&ast.SelectorExpr{
								X:   &ast.Ident{Name: stOutRefName},
								Sel: &ast.Ident{Name: in.Names[0].Name},
							},
						},
						Tok: token.ASSIGN,
						Rhs: []ast.Expr{&ast.CallExpr{
							Fun: &ast.Ident{Name: "append"},
							Args: []ast.Expr{
								&ast.SelectorExpr{
									X:   &ast.Ident{Name: stOutRefName},
									Sel: &ast.Ident{Name: in.Names[0].Name},
								},
								&ast.Ident{Name: "temp"},
							},
						}},
					},
				},
			},
		}
		bodyList = append(bodyList, rangeStmt)
	case "Path2F":
		ParamMap := map[string]string{"X": "0", "Y": "1"}
		ifStruct := ListGetPathUtilToProto(ParamMap, in.Names[0].Name, tp, "Fvec2", "2")
		bodyList = append(bodyList, ifStruct)
	case "Path3F":
		ParamMap := map[string]string{"X": "0", "Y": "1", "Z": "2"}
		ifStruct := ListGetPathUtilToProto(ParamMap, in.Names[0].Name, tp, "Fvec3", "3")
		bodyList = append(bodyList, ifStruct)
	case "Path2D":
		ParamMap := map[string]string{"X": "0", "Y": "1"}
		ifStruct := ListGetPathUtilToProto(ParamMap, in.Names[0].Name, tp, "Dvec2", "2")
		bodyList = append(bodyList, ifStruct)
	case "Path3D":
		ParamMap := map[string]string{"X": "0", "Y": "1", "Z": "2"}
		ifStruct := ListGetPathUtilToProto(ParamMap, in.Names[0].Name, tp, "Dvec3", "3")
		bodyList = append(bodyList, ifStruct)
	}
	return bodyList
}

func ListGetVecUtilToProto(m map[string]string, name, st, num string) *ast.RangeStmt {
	// value
	var ParamList []ast.Stmt
	for k, v := range m {
		temp := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: "temp"},
					Sel: &ast.Ident{Name: k},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{
				&ast.IndexExpr{
					X:     &ast.Ident{Name: "v"},
					Index: &ast.BasicLit{Value: v, Kind: token.INT},
				},
			},
		}
		ParamList = append(ParamList, temp)
	}
	ifStmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X: &ast.CallExpr{
				Fun: &ast.Ident{
					Name: "len",
				},
				Args: []ast.Expr{&ast.Ident{Name: "v"}},
			},
			Op: token.EQ,
			Y:  &ast.BasicLit{Value: num, Kind: token.INT},
		},
		Body: &ast.BlockStmt{
			List: ParamList,
		},
	}
	rangeStmt := &ast.RangeStmt{
		Key:   &ast.Ident{Name: "_"},
		Value: &ast.Ident{Name: "v"},
		Tok:   token.DEFINE,
		X: &ast.SelectorExpr{
			X:   &ast.Ident{Name: stInRefName},
			Sel: &ast.Ident{Name: name},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: "temp"}},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{&ast.UnaryExpr{
						Op: token.AND,
						X: &ast.CompositeLit{Type: &ast.SelectorExpr{
							X:   &ast.Ident{Name: "DslBase"},
							Sel: &ast.Ident{Name: st},
						}, Incomplete: false},
					}},
				},
				ifStmt,
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   &ast.Ident{Name: stOutRefName},
							Sel: &ast.Ident{Name: name},
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun: &ast.Ident{Name: "append"},
						Args: []ast.Expr{
							&ast.SelectorExpr{
								X:   &ast.Ident{Name: stOutRefName},
								Sel: &ast.Ident{Name: name},
							},
							&ast.Ident{Name: "temp"},
						},
					}},
				},
			},
		},
	}
	return rangeStmt
}

func ListGetPathUtilToProto(m map[string]string, name, st, st2, num string) *ast.RangeStmt {
	var ParamList []ast.Stmt
	for k, v := range m {
		temp := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: "temp"},
					Sel: &ast.Ident{Name: k},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{
				&ast.IndexExpr{
					X:     &ast.Ident{Name: "v1"},
					Index: &ast.BasicLit{Value: v, Kind: token.INT},
				},
			},
		}
		ParamList = append(ParamList, temp)
	}
	rangeStmt := &ast.RangeStmt{
		Key:   &ast.Ident{Name: "_"},
		Value: &ast.Ident{Name: "v1"},
		Tok:   token.DEFINE,
		X:     &ast.Ident{Name: "v"},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: "temp"}},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{&ast.UnaryExpr{
						Op: token.AND,
						X: &ast.CompositeLit{Type: &ast.SelectorExpr{
							X:   &ast.Ident{Name: "DslBase"},
							Sel: &ast.Ident{Name: st2},
						}, Incomplete: false},
					}},
				},
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{
						X: &ast.CallExpr{
							Fun:  &ast.Ident{Name: "len"},
							Args: []ast.Expr{&ast.Ident{Name: "v1"}},
						},
						Op: token.EQ,
						Y:  &ast.BasicLit{Value: num, Kind: token.INT},
					},
					Body: &ast.BlockStmt{
						List: ParamList,
					},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   &ast.Ident{Name: "op"},
							Sel: &ast.Ident{Name: "Points"},
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun: &ast.Ident{Name: "append"},
						Args: []ast.Expr{
							&ast.SelectorExpr{
								X:   &ast.Ident{Name: "op"},
								Sel: &ast.Ident{Name: "Points"},
							},
							&ast.Ident{Name: "temp"},
						},
					}},
				},
			},
		},
	}
	rangeStmtOut := &ast.RangeStmt{
		Key:   &ast.Ident{Name: "_"},
		Value: &ast.Ident{Name: "v"},
		Tok:   token.DEFINE,
		X: &ast.SelectorExpr{
			X:   &ast.Ident{Name: stInRefName},
			Sel: &ast.Ident{Name: name},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: "op"}},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{&ast.UnaryExpr{
						Op: token.AND,
						X: &ast.CompositeLit{Type: &ast.SelectorExpr{
							X:   &ast.Ident{Name: "DslBase"},
							Sel: &ast.Ident{Name: st},
						}, Incomplete: false},
					}},
				},
				rangeStmt,
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   &ast.Ident{Name: stOutRefName},
							Sel: &ast.Ident{Name: name},
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun: &ast.Ident{Name: "append"},
						Args: []ast.Expr{
							&ast.SelectorExpr{
								X:   &ast.Ident{Name: stOutRefName},
								Sel: &ast.Ident{Name: name},
							},
							&ast.Ident{Name: "op"},
						},
					}},
				},
			},
		},
	}

	return rangeStmtOut
}

func GetDslName(in string) (out string) {
	out = "Dsl" + in
	return
}

func RemoveDslName(in string) (out string) {
	out = in
	if len(in) > 3 && in[:3] == "Dsl" {
		out = in[3:]
	}
	return
}

func SingleArrayType(arr *ast.ArrayType) bool {
	if _, ok := arr.Elt.(*ast.Ident); ok {
		return true
	} else if v, ok := arr.Elt.(*ast.ArrayType); ok {
		return SingleArrayType(v)
	} else {
		return false
	}
}
