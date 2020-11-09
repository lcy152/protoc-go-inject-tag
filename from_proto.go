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

func CreateFromProtoFunc(inputPath string, config Config) (err error) {
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
			// typeSpec.Name.Name = GetDslName(typeSpec.Name.Name)
			originName := RemoveDslName(typeSpec.Name.Name)
			structNameList = append(structNameList, structOriginName)

			structDecl, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}

			FromProtoFunc := &ast.FuncDecl{
				Type: &ast.FuncType{
					Params: &ast.FieldList{
						List: []*ast.Field{
							&ast.Field{
								Names: []*ast.Ident{
									&ast.Ident{Name: stOutRefName},
								},
								Type: &ast.StarExpr{X: &ast.Ident{Name: originName}},
							},
						},
					},
				},
				Recv: &ast.FieldList{
					List: []*ast.Field{
						&ast.Field{
							Names: []*ast.Ident{
								&ast.Ident{Name: stInRefName},
							},
							Type: &ast.StarExpr{
								X: &ast.Ident{Name: structOriginName},
							},
						},
					},
				},
				Name: &ast.Ident{Name: "FromProto"},
			}
			var bodyList []ast.Stmt
			IfStruct := &ast.IfStmt{
				Cond: &ast.BinaryExpr{
					X: &ast.Ident{
						Name: stOutRefName,
					},
					Op: token.EQ,
					Y:  &ast.BasicLit{Value: "nil"},
				},
				Body: &ast.BlockStmt{
					List: []ast.Stmt{
						&ast.ReturnStmt{},
					},
				},
			}
			bodyList = append(bodyList, IfStruct)
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
					if (len(field.Names[0].Name) > 3 && field.Names[0].Name[0:4] == "XXX_") || field.Names[0].Name == DslTypeHash || field.Names[0].Name == DslTypeVersion {
						continue
					}
					SetSingleValue := func(field *ast.Field) {
						leftSelector := &ast.SelectorExpr{}
						rightSelector := &ast.SelectorExpr{}
						leftSelector.X = &ast.Ident{Name: stInRefName}
						rightSelector.X = &ast.Ident{Name: stOutRefName}
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
												bodyList = ListBaseTypeTrandformFromProto(field, t2.Sel.Name, bodyList)
											}
										}
									}
								}
							}
							if !isBaseType {
								vartype := ""
								if t, ok := field.Type.(*ast.ArrayType); ok {
									if t1, ok := t.Elt.(*ast.StarExpr); ok {
										if t2, ok := t1.X.(*ast.Ident); ok {
											vartype = t2.Name
										}
									}
								}
								rangeItem := &ast.RangeStmt{
									Key:   &ast.Ident{Name: "_"},
									Value: &ast.Ident{Name: "v"},
									Tok:   token.DEFINE,
									X: &ast.SelectorExpr{
										X:   &ast.Ident{Name: stOutRefName},
										Sel: &ast.Ident{Name: field.Names[0].Name},
									},
									Body: &ast.BlockStmt{
										List: []ast.Stmt{
											&ast.AssignStmt{
												Lhs: []ast.Expr{&ast.Ident{Name: "temp"}},
												Tok: token.DEFINE,
												Rhs: []ast.Expr{&ast.UnaryExpr{
													Op: token.AND,
													X: &ast.CompositeLit{
														Type:       &ast.Ident{Name: vartype},
														Incomplete: false,
													},
												}},
											},
											&ast.ExprStmt{
												X: &ast.CallExpr{
													Fun: &ast.SelectorExpr{
														X:   &ast.Ident{Name: "temp"},
														Sel: &ast.Ident{Name: "FromProto"},
													},
													Args: []ast.Expr{
														&ast.Ident{Name: "v"},
													},
												},
											},
											&ast.AssignStmt{
												Lhs: []ast.Expr{
													&ast.SelectorExpr{
														X:   &ast.Ident{Name: stInRefName},
														Sel: &ast.Ident{Name: field.Names[0].Name},
													},
												},
												Tok: token.ASSIGN,
												Rhs: []ast.Expr{&ast.CallExpr{
													Fun: &ast.Ident{Name: "append"},
													Args: []ast.Expr{
														&ast.SelectorExpr{
															X:   &ast.Ident{Name: stInRefName},
															Sel: &ast.Ident{Name: field.Names[0].Name},
														},
														&ast.Ident{Name: "temp"},
													},
												}},
											},
										},
									},
								}
								setNil := &ast.AssignStmt{
									Lhs: []ast.Expr{
										&ast.SelectorExpr{
											X:   &ast.Ident{Name: stInRefName},
											Sel: &ast.Ident{Name: field.Names[0].Name},
										},
									},
									Tok: token.ASSIGN,
									Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
								}
								bodyList = append(bodyList, setNil, rangeItem)
								// if arrIn, ok := arr.Elt.(*ast.StarExpr); ok {
								// 	if arrInIn, ok := arrIn.X.(*ast.Ident); ok {
								// 		arrInIn.Name = GetDslName(arrInIn.Name)
								// 	}
								// }

							}
						}
					} else {
						isBaseType := false
						if t, ok := field.Type.(*ast.StarExpr); ok {
							if t1, ok := t.X.(*ast.SelectorExpr); ok {
								if t2, ok := t1.X.(*ast.Ident); ok {
									if "DslBase" == t2.Name {
										isBaseType = true
										bodyList = BaseTypeTrandformFromProto(field, t1.Sel.Name, bodyList)
										continue
									}
								}
							}
						}
						if !isBaseType {
							vartype := ""
							if t, ok := field.Type.(*ast.StarExpr); ok {
								if t1, ok := t.X.(*ast.Ident); ok {
									vartype = t1.Name
								}
							}
							body := &ast.ExprStmt{
								X: &ast.CallExpr{
									Fun: &ast.SelectorExpr{
										X: &ast.SelectorExpr{
											X:   &ast.Ident{Name: stInRefName},
											Sel: &ast.Ident{Name: field.Names[0].Name},
										},
										Sel: &ast.Ident{Name: "FromProto"},
									},
									Args: []ast.Expr{
										&ast.SelectorExpr{
											X:   &ast.Ident{Name: stOutRefName},
											Sel: &ast.Ident{Name: field.Names[0].Name},
										},
									},
								},
							}
							isIfStruct := &ast.IfStmt{
								Cond: &ast.BinaryExpr{
									X: &ast.SelectorExpr{
										X: &ast.Ident{
											Name: stOutRefName,
										},
										Sel: &ast.Ident{
											Name: field.Names[0].Name,
										},
									},
									Op: token.NEQ,
									Y:  &ast.BasicLit{Value: "nil"},
								},
								Body: &ast.BlockStmt{
									List: []ast.Stmt{
										&ast.AssignStmt{
											Lhs: []ast.Expr{&ast.SelectorExpr{
												X:   &ast.Ident{Name: stInRefName},
												Sel: &ast.Ident{Name: field.Names[0].Name},
											}},
											Tok: token.ASSIGN,
											Rhs: []ast.Expr{&ast.UnaryExpr{
												Op: token.AND,
												X: &ast.CompositeLit{
													Type:       &ast.Ident{Name: vartype},
													Incomplete: false,
												},
											}},
										},
										body,
									},
								},
							}
							bodyList = append(bodyList, isIfStruct)
							// if arrIn, ok := field.Type.(*ast.StarExpr); ok {
							// 	if arrInIn, ok := arrIn.X.(*ast.Ident); ok {
							// 		arrInIn.Name = GetDslName(arrInIn.Name)
							// 	}
							// }
						}
					}
				}
			}
			funcBody := &ast.BlockStmt{}
			funcBody.List = bodyList
			FromProtoFunc.Body = funcBody
			f.Decls = append(f.Decls, FromProtoFunc)
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

func BaseTypeTrandformFromProto(in *ast.Field, tp string, bodyList []ast.Stmt) []ast.Stmt {
	switch tp {
	case "TimeStamp":
		isIfStruct := &ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X: &ast.SelectorExpr{
					X:   &ast.Ident{Name: stOutRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
				Op: token.NEQ,
				Y:  &ast.BasicLit{Value: "nil"},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.AssignStmt{
						Lhs: []ast.Expr{
							&ast.SelectorExpr{
								X:   &ast.Ident{Name: stInRefName},
								Sel: &ast.Ident{Name: in.Names[0].Name},
							},
						},
						Tok: token.ASSIGN,
						Rhs: []ast.Expr{
							&ast.CallExpr{
								Fun: &ast.Ident{
									Name: "primitive.DateTime",
								},
								Args: []ast.Expr{
									&ast.SelectorExpr{
										X: &ast.SelectorExpr{
											X:   &ast.Ident{Name: stOutRefName},
											Sel: &ast.Ident{Name: in.Names[0].Name},
										},
										Sel: &ast.Ident{Name: "Time"},
									},
								},
							},
						},
					},
				},
			},
		}
		bodyList = append(bodyList, isIfStruct)
	case "Ivec2", "Fvec2", "Dvec2":
		ParamMap := []string{"X", "Y"}
		ifStruct := GetVecUtilFromProto(ParamMap, in.Names[0].Name)
		bodyList = append(bodyList, ifStruct)
	case "Ivec3", "Fvec3", "Dvec3":
		ParamMap := []string{"X", "Y", "Z"}
		ifStruct := GetVecUtilFromProto(ParamMap, in.Names[0].Name)
		bodyList = append(bodyList, ifStruct)
	case "Dmat3", "Dmat4":
		isIfStruct := &ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X: &ast.SelectorExpr{
					X:   &ast.Ident{Name: stOutRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
				Op: token.NEQ,
				Y:  &ast.BasicLit{Value: "nil"},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.AssignStmt{
						Lhs: []ast.Expr{
							&ast.SelectorExpr{
								X:   &ast.Ident{Name: stInRefName},
								Sel: &ast.Ident{Name: in.Names[0].Name},
							},
						},
						Tok: token.ASSIGN,
						Rhs: []ast.Expr{
							&ast.SelectorExpr{
								X: &ast.SelectorExpr{
									X:   &ast.Ident{Name: stOutRefName},
									Sel: &ast.Ident{Name: in.Names[0].Name},
								},
								Sel: &ast.Ident{Name: "Data"},
							},
						},
					},
				},
			},
		}
		bodyList = append(bodyList, isIfStruct)
	case "Path2F":
		ParamMap := []string{"X", "Y"}
		ifStruct := GetPathUtilFromProto(ParamMap, in.Names[0].Name, tp, "float32", "2")
		setNil := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: stInRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
		}
		bodyList = append(bodyList, setNil, ifStruct)
	case "Path3F":
		ParamMap := []string{"X", "Y", "Z"}
		ifStruct := GetPathUtilFromProto(ParamMap, in.Names[0].Name, tp, "float32", "3")
		setNil := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: stInRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
		}
		bodyList = append(bodyList, setNil, ifStruct)
	case "Path2D":
		ParamMap := []string{"X", "Y"}
		ifStruct := GetPathUtilFromProto(ParamMap, in.Names[0].Name, tp, "float64", "2")
		setNil := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: stInRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
		}
		bodyList = append(bodyList, setNil, ifStruct)
	case "Path3D":
		ParamMap := []string{"X", "Y", "Z"}
		ifStruct := GetPathUtilFromProto(ParamMap, in.Names[0].Name, tp, "float64", "3")

		setNil := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: stInRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
		}
		bodyList = append(bodyList, setNil, ifStruct)
	}
	return bodyList
}

func GetVecUtilFromProto(m []string, name string) *ast.IfStmt {
	temp := &ast.AssignStmt{}
	tArgs := []ast.Expr{}
	tArgs = append(tArgs, &ast.SelectorExpr{
		X:   &ast.Ident{Name: stInRefName},
		Sel: &ast.Ident{Name: name},
	})
	for _, k := range m {
		tArgs = append(tArgs, &ast.SelectorExpr{
			X: &ast.SelectorExpr{
				X:   &ast.Ident{Name: stOutRefName},
				Sel: &ast.Ident{Name: name},
			},
			Sel: &ast.Ident{Name: k},
		})
	}
	temp = &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.SelectorExpr{
				X:   &ast.Ident{Name: stInRefName},
				Sel: &ast.Ident{Name: name},
			},
		},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{&ast.CallExpr{
			Fun:  &ast.Ident{Name: "append"},
			Args: tArgs,
		}},
	}

	setNil := &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.SelectorExpr{
				X:   &ast.Ident{Name: stInRefName},
				Sel: &ast.Ident{Name: name},
			},
		},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
	}

	isIfStruct := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X: &ast.SelectorExpr{
				X: &ast.Ident{
					Name: stOutRefName,
				},
				Sel: &ast.Ident{
					Name: name,
				},
			},
			Op: token.NEQ,
			Y:  &ast.BasicLit{Value: "nil"},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{setNil, temp},
		},
	}
	return isIfStruct
}

func GetPathUtilFromProto(m []string, name, st, typeGuild, num string) *ast.IfStmt {
	var ParamList []ast.Expr
	for _, k := range m {
		temp := &ast.SelectorExpr{
			X:   &ast.Ident{Name: "v"},
			Sel: &ast.Ident{Name: k},
		}
		ParamList = append(ParamList, temp)
	}
	rangeStmt := &ast.RangeStmt{
		Key:   &ast.Ident{Name: "_"},
		Value: &ast.Ident{Name: "v"},
		Tok:   token.DEFINE,
		X: &ast.SelectorExpr{
			X: &ast.SelectorExpr{
				X:   &ast.Ident{Name: stOutRefName},
				Sel: &ast.Ident{Name: name},
			},
			Sel: &ast.Ident{Name: "Points"},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{&ast.Ident{Name: "temp"}},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{&ast.CompositeLit{
						Type: &ast.ArrayType{
							Elt: &ast.Ident{Name: typeGuild},
						},
						Elts: ParamList,
					}},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   &ast.Ident{Name: stInRefName},
							Sel: &ast.Ident{Name: name},
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun: &ast.Ident{Name: "append"},
						Args: []ast.Expr{
							&ast.SelectorExpr{
								X:   &ast.Ident{Name: stInRefName},
								Sel: &ast.Ident{Name: name},
							},
							&ast.Ident{Name: "temp"},
						},
					}},
				},
			},
		},
	}
	isIfStruct := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X: &ast.SelectorExpr{
				X: &ast.Ident{
					Name: stOutRefName,
				},
				Sel: &ast.Ident{
					Name: name,
				},
			},
			Op: token.NEQ,
			Y:  &ast.BasicLit{Value: "nil"},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{rangeStmt},
		},
	}

	return isIfStruct
}

func ListBaseTypeTrandformFromProto(in *ast.Field, tp string, bodyList []ast.Stmt) []ast.Stmt {
	switch tp {
	case "Ivec2":
		ParamMap := []string{"X", "Y"}
		ifStruct := ListGetVecUtilFromProto(ParamMap, in.Names[0].Name, "int32", "2")
		bodyList = append(bodyList, ifStruct)
	case "Ivec3":
		ParamMap := []string{"X", "Y", "Z"}
		ifStruct := ListGetVecUtilFromProto(ParamMap, in.Names[0].Name, "int32", "3")
		bodyList = append(bodyList, ifStruct)
	case "Fvec2":
		ParamMap := []string{"X", "Y"}
		ifStruct := ListGetVecUtilFromProto(ParamMap, in.Names[0].Name, "float32", "2")
		bodyList = append(bodyList, ifStruct)
	case "Fvec3":
		ParamMap := []string{"X", "Y", "Z"}
		ifStruct := ListGetVecUtilFromProto(ParamMap, in.Names[0].Name, "float32", "3")
		bodyList = append(bodyList, ifStruct)
	case "Dvec2":
		ParamMap := []string{"X", "Y"}
		ifStruct := ListGetVecUtilFromProto(ParamMap, in.Names[0].Name, "float64", "2")
		bodyList = append(bodyList, ifStruct)
	case "Dvec3":
		ParamMap := []string{"X", "Y", "Z"}
		ifStruct := ListGetVecUtilFromProto(ParamMap, in.Names[0].Name, "float64", "3")
		bodyList = append(bodyList, ifStruct)
	case "Dmat3", "Dmat4":
		rangeStmt := &ast.RangeStmt{
			Key:   &ast.Ident{Name: "_"},
			Value: &ast.Ident{Name: "v"},
			Tok:   token.DEFINE,
			X: &ast.SelectorExpr{
				X:   &ast.Ident{Name: stOutRefName},
				Sel: &ast.Ident{Name: in.Names[0].Name},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					&ast.AssignStmt{
						Lhs: []ast.Expr{
							&ast.SelectorExpr{
								X:   &ast.Ident{Name: stInRefName},
								Sel: &ast.Ident{Name: in.Names[0].Name},
							},
						},
						Tok: token.ASSIGN,
						Rhs: []ast.Expr{&ast.CallExpr{
							Fun: &ast.Ident{Name: "append"},
							Args: []ast.Expr{
								&ast.SelectorExpr{
									X:   &ast.Ident{Name: stInRefName},
									Sel: &ast.Ident{Name: in.Names[0].Name},
								},
								&ast.SelectorExpr{
									X:   &ast.Ident{Name: "v"},
									Sel: &ast.Ident{Name: "Data"},
								},
							},
						}},
					},
				},
			},
		}

		setNil := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: stInRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
		}
		isIfStruct := &ast.IfStmt{
			Cond: &ast.BinaryExpr{
				X: &ast.SelectorExpr{
					X:   &ast.Ident{Name: stOutRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
				Op: token.NEQ,
				Y:  &ast.BasicLit{Value: "nil"},
			},
			Body: &ast.BlockStmt{
				List: []ast.Stmt{
					setNil,
					rangeStmt,
				},
			},
		}
		bodyList = append(bodyList, isIfStruct)
	case "Path2F":
		ParamMap := []string{"X", "Y"}
		ifStruct := ListGetPathUtilFromProto(ParamMap, in.Names[0].Name, tp, "float32", "2")
		setNil := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: stInRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
		}
		bodyList = append(bodyList, setNil, ifStruct)
	case "Path3F":
		ParamMap := []string{"X", "Y", "Z"}
		ifStruct := ListGetPathUtilFromProto(ParamMap, in.Names[0].Name, tp, "float32", "3")
		setNil := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: stInRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
		}
		bodyList = append(bodyList, setNil, ifStruct)
	case "Path2D":
		ParamMap := []string{"X", "Y"}
		ifStruct := ListGetPathUtilFromProto(ParamMap, in.Names[0].Name, tp, "float64", "2")
		setNil := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: stInRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
		}
		bodyList = append(bodyList, setNil, ifStruct)
	case "Path3D":
		ParamMap := []string{"X", "Y", "Z"}
		ifStruct := ListGetPathUtilFromProto(ParamMap, in.Names[0].Name, tp, "float64", "3")
		setNil := &ast.AssignStmt{
			Lhs: []ast.Expr{
				&ast.SelectorExpr{
					X:   &ast.Ident{Name: stInRefName},
					Sel: &ast.Ident{Name: in.Names[0].Name},
				},
			},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
		}
		bodyList = append(bodyList, setNil, ifStruct)
	}
	return bodyList
}

func ListGetVecUtilFromProto(m []string, name, st, num string) *ast.IfStmt {
	// value
	var temp []ast.Expr
	for _, k := range m {
		temp = append(temp, &ast.SelectorExpr{
			X:   &ast.Ident{Name: "v"},
			Sel: &ast.Ident{Name: k},
		})
	}

	rangeStmt := &ast.RangeStmt{
		Key:   &ast.Ident{Name: "_"},
		Value: &ast.Ident{Name: "v"},
		Tok:   token.DEFINE,
		X: &ast.SelectorExpr{
			X:   &ast.Ident{Name: stOutRefName},
			Sel: &ast.Ident{Name: name},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.Ident{Name: "temp"},
					},
					Tok: token.DEFINE,
					Rhs: []ast.Expr{
						&ast.CompositeLit{
							Type: &ast.ArrayType{
								Elt: &ast.Ident{Name: st},
							},
							Elts: temp,
						},
					},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   &ast.Ident{Name: stInRefName},
							Sel: &ast.Ident{Name: name},
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun: &ast.Ident{Name: "append"},
						Args: []ast.Expr{
							&ast.SelectorExpr{
								X:   &ast.Ident{Name: stInRefName},
								Sel: &ast.Ident{Name: name},
							},
							&ast.Ident{Name: "temp"},
						},
					}},
				},
			},
		},
	}
	setNil := &ast.AssignStmt{
		Lhs: []ast.Expr{
			&ast.SelectorExpr{
				X:   &ast.Ident{Name: stInRefName},
				Sel: &ast.Ident{Name: name},
			},
		},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{&ast.Ident{Name: "nil"}},
	}
	isIfStruct := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			X: &ast.SelectorExpr{
				X: &ast.Ident{
					Name: stOutRefName,
				},
				Sel: &ast.Ident{
					Name: name,
				},
			},
			Op: token.NEQ,
			Y:  &ast.BasicLit{Value: "nil"},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{setNil, rangeStmt},
		},
	}

	return isIfStruct
}

func ListGetPathUtilFromProto(m []string, name, st, st2, num string) *ast.RangeStmt {
	var temp []ast.Expr
	for _, k := range m {
		temp = append(temp, &ast.SelectorExpr{
			X:   &ast.Ident{Name: "v1"},
			Sel: &ast.Ident{Name: k},
		})
	}

	rangeStmt := &ast.RangeStmt{
		Key:   &ast.Ident{Name: "_"},
		Value: &ast.Ident{Name: "v"},
		Tok:   token.DEFINE,
		X: &ast.SelectorExpr{
			X:   &ast.Ident{Name: stOutRefName},
			Sel: &ast.Ident{Name: name},
		},
		Body: &ast.BlockStmt{
			List: []ast.Stmt{
				&ast.DeclStmt{
					Decl: &ast.GenDecl{
						Tok: token.VAR,
						Specs: []ast.Spec{
							&ast.ValueSpec{
								Names: []*ast.Ident{&ast.Ident{Name: "temp"}},
								Type: &ast.ArrayType{
									Elt: &ast.ArrayType{
										Elt: &ast.Ident{Name: st2},
									},
								},
							},
						},
					},
				},
				&ast.RangeStmt{
					Key:   &ast.Ident{Name: "_"},
					Value: &ast.Ident{Name: "v1"},
					Tok:   token.DEFINE,
					X: &ast.SelectorExpr{
						X:   &ast.Ident{Name: "v"},
						Sel: &ast.Ident{Name: "Points"},
					},
					Body: &ast.BlockStmt{
						List: []ast.Stmt{
							&ast.AssignStmt{
								Lhs: []ast.Expr{
									&ast.Ident{Name: "temp2"},
								},
								Tok: token.DEFINE,
								Rhs: []ast.Expr{
									&ast.CompositeLit{
										Type: &ast.ArrayType{
											Elt: &ast.Ident{Name: st2},
										},
										Elts: temp,
									},
								},
							},
							&ast.AssignStmt{
								Lhs: []ast.Expr{
									&ast.Ident{Name: "temp"},
								},
								Tok: token.ASSIGN,
								Rhs: []ast.Expr{&ast.CallExpr{
									Fun: &ast.Ident{Name: "append"},
									Args: []ast.Expr{
										&ast.Ident{Name: "temp"},
										&ast.Ident{Name: "temp2"},
									},
								}},
							},
						},
					},
				},
				&ast.AssignStmt{
					Lhs: []ast.Expr{
						&ast.SelectorExpr{
							X:   &ast.Ident{Name: stInRefName},
							Sel: &ast.Ident{Name: name},
						},
					},
					Tok: token.ASSIGN,
					Rhs: []ast.Expr{&ast.CallExpr{
						Fun: &ast.Ident{Name: "append"},
						Args: []ast.Expr{
							&ast.SelectorExpr{
								X:   &ast.Ident{Name: stInRefName},
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
