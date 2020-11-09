package main

import "regexp"

var (
	rComment = regexp.MustCompile(`^//\s*@inject_tag:\s*(.*)$`)
	rInject  = regexp.MustCompile("`.+`$")
	rTags    = regexp.MustCompile(`[\w_]+:"[^"]+"`)
)

type textArea struct {
	Start       int
	End         int
	CurrentTag  string
	InjectTag   string
	CurrentType string
	InjectType  string
	CurrentName string
	InjectName  string
}

type TypeConfig struct {
	Target          string   `json:"target"`
	Replace         string   `json:"replace"`
	RemoveOmitempty bool     `json:"remove_omitempty"`
	ImportList      []string `json:"import_list"`
}

type TagConfig struct {
	Target  string `json:"target"`
	Replace string `json:"replace"`
}

type NameConfig struct {
	RepeatName string `json:"repeat_name"`
	ReName     string `json:"rename"`
}

type RegularConfig struct {
	TypeList    []TypeConfig `json:"type"`
	BsonTagList []TagConfig  `json:"bson_tag"`
	NameList    []NameConfig `json:"rename"`
}

type Config struct {
	Type map[string]TypeConfig
	Tag  map[string]string
	Name map[string]string
}

type ProtoI struct {
	HashName    string
	VersionName string
}

type ProtoHash struct {
	HashMap    map[string]string
	VersionMap map[string]int32
	NameMap    map[string]ProtoI
	PathMap    map[string]string
}
