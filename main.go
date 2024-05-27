package main

import (
	"fmt"
	"github.com/BurntSushi/toml"
	"golang.org/x/crypto/ssh"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
)

var (
	mysqlDb  *gorm.DB
	Configer map[string]*ConfigInfo
	dial     *ssh.Client
	err      error
	table    string
)

type ConfigInfo struct {
	Pack     string `toml:"pack"`
	User     string `toml:"user"`
	Host     string `toml:"host"`
	Database string `toml:"database"`
	Pass     string `toml:"pass"`
	SaveDir  string `toml:"saveDir"`
	Type     string `toml:"type"`
}

func updateConfig() {
	_, err = toml.DecodeFile("config.toml", &Configer)
	if err != nil {
		panic("读取配置文件失败!,原因:" + err.Error())
	}
}

func main() {
	updateConfig()
	var project, temp string
	var title = "请选择项目:"
	for name := range Configer {
		title += name + " "
	}
	for {
		fmt.Println(title)
		fmt.Scanln(&project)
		fmt.Println("选择项目:", project)
		if project == "" && temp != "" {
			project = temp
		}
		updateConfig()
		conf, ok := Configer[project]
		if !ok {
			continue
		}
		temp = project
		localConnect(conf)
		fmt.Println("enter默认全部，或 请输入表名：")
		fmt.Scanln(&table)
		execSql(conf)
		title = "请选择项目:app yk local_yk,回复空格使用上一个"
	}
}

func localConnect(conf *ConfigInfo) bool {
	if conf == nil {
		println("config is nil")
		return false
	}
	dsn := fmt.Sprintf(
		"%s:%s@%s(%s)/%s?charset=utf8mb4&multiStatements=true&parseTime=True&loc=Local",
		conf.User,
		conf.Pass,
		conf.Type,
		conf.Host,
		conf.Database,
	)
	mysqlDb, err = gorm.Open(mysql.Open(dsn))
	if err != nil {
		panic("mysql 启动失败!,原因:" + err.Error())
	}
	if err != nil {
		fmt.Println(err)
	}
	return true
}

func execSql(conf *ConfigInfo) {
	isSqlBit = false
	type tabField struct {
		Name    string `gorm:"column:Name"`
		Comment string `gorm:"column:Comment"`
	}
	sql := "show table status;"
	var tableObj []tabField
	mysqlDb.Raw(sql).Scan(&tableObj)
	var tableName = make(map[string]string, len(tableObj)) //表名
	for _, v := range tableObj {
		if v.Comment == "" {
			continue
		}
		content := strings.ReplaceAll(v.Comment, "\r", "")
		content = strings.ReplaceAll(content, "\n", "")
		tableName[v.Name] = fmt.Sprintf("\n//%s  %s", v.Name, content)
	}
	if table == "" {
		//if err = os.RemoveAll(conf.SaveDir); err == nil {
		//	os.Mkdir(conf.SaveDir, 0777)
		//}

		type Field struct {
			Field string `gorm:"column:table_name"`
		}
		var fieldObj []Field
		sql = fmt.Sprintf("select table_name as table_name  from INFORMATION_SCHEMA.TABLES where table_schema = '%s'", conf.Database)
		mysqlDb.Raw(sql).Scan(&fieldObj)
		for _, v := range fieldObj {
			ToAFile(conf, v.Field, tableName)
		}
		return
	}
	ToAFile(conf, table, tableName)
}

func execCmd(order string) {
	cmd := exec.Command("powershell", order)
	_, err := cmd.Output()
	if err != nil {
		println(err.Error())
		if es, ok := err.(*exec.ExitError); ok {
			println(string(es.Stderr))
		}
	}
}

func ToAFile(conf *ConfigInfo, table string, tabMap map[string]string) {
	type Field struct {
		Field   string `gorm:"column:Field"`
		Type    string `gorm:"column:Type"`
		Comment string `gorm:"column:Comment"`
	}
	importTime := ""
	var fieldObj []Field
	fields := ""
	sql := fmt.Sprintf("SELECT TABLE_NAME, COLUMN_COMMENT as Comment, COLUMN_NAME as Field, DATA_TYPE as Type "+
		"from INFORMATION_SCHEMA.COLUMNS WHERE TABLE_NAME='%s' and TABLE_SCHEMA = '%s'  ORDER BY ORDINAL_POSITION ASC;", table, conf.Database)
	mysqlDb.Raw(sql).Scan(&fieldObj)
	var length int
	for _, value := range fieldObj {
		if len(value.Field) > length {
			length = len(value.Field)
		}
	}
	var hasTime bool
	var space = "                                                                                                      "
	for _, value := range fieldObj {
		spaces := space[:length-len(value.Field)]
		types := getType(value.Type)
		switch {
		case !hasTime && types == "*time.Time":
			importTime = `import "time"`
			hasTime = true
		case types == "SqlBit" && !isSqlBit:
			f, e := os.Create(path.Join(conf.SaveDir, "bit.go"))
			if e != nil {
				fmt.Println("打开文件错误", e)
				return
			}
			_, we := io.WriteString(f, "package "+conf.Pack+sqlBit)
			if we != nil {
				fmt.Println("写入文件错误", we)
				return
			}
			f.Close()
			isSqlBit = true
		}

		content := strings.ReplaceAll(value.Comment, "\r", "")
		content = strings.ReplaceAll(content, "\n", "")
		field := toUp(value.Field)
		if field == "TableName" {
			field += "s"
		}
		fields += fmt.Sprintf("\n\t %s %s `gorm:\"column:%s\" %sdesc:\"%s\"`", field, types, value.Field, spaces, content)
	}

	infos := fmt.Sprintf(`package %s
%s
%s
type %s struct {%s
}
`, conf.Pack, importTime, tabMap[table], toUp(table), fields)

	infos += fmt.Sprintf(`
func (%s) TableName() string {
	return "%s"
}`, toUp(table), table)

	fileName := path.Join(conf.SaveDir, table+".go")
	f, e := os.Create(fileName)
	if e != nil {
		fmt.Println("打开文件错误", e)
		return
	}
	_, we := io.WriteString(f, infos)
	if we != nil {
		fmt.Println("写入文件错误", we)
		return
	}
	f.Close()
}

func toUp(field string) string {
	var nextUp bool
	str := ""
	for key, value := range field {
		if key == 0 {
			str = str + strings.ToUpper(string(value))
			continue
		}
		if string(value) == "_" {
			nextUp = true
			continue
		}
		if nextUp {
			str = str + strings.ToUpper(string(value))
			nextUp = false
		} else {
			str = str + string(value)
		}
	}

	return str

}

var filterMap = map[string]string{
	"tinyint":    "int64",
	"smallint":   "int64",
	"mediumint":  "int64",
	"int":        "int64",
	"bigint":     "int64",
	"float":      "float64",
	"double":     "float64",
	"decimal":    "float64",
	"year":       "string",
	"time":       "string",
	"date":       "*time.Time",
	"datetime":   "*time.Time",
	"timestamp":  "*time.Time",
	"char":       "string",
	"varchar":    "string",
	"blob":       "[]byte",
	"tinytext":   "string",
	"text":       "string",
	"mediumtext": "string",
	"longtext":   "string",
	"enum":       "string",
	"bit":        "SqlBit",
	"boolean":    "SqlBit",
	"json":       "*string",
	// 其他类型默认转字符
}

func getType(typeString string) string {

	if val, ex := filterMap[typeString]; ex {
		return val
	} else {
		return "string"
	}
}

var (
	isSqlBit bool
	sqlBit   = `

type SqlBit []uint8

// bit 转int64
func (s SqlBit) GetInt64() int64 {
	if len(s) != 8 {
		return 0
	}
	return int64(s[7]) | int64(s[6])<<8 | int64(s[5])<<16 | int64(s[4])<<24 |
		int64(s[3])<<32 | int64(s[2])<<40 | int64(s[1])<<48 | int64(s[0])<<56
}

func (s SqlBit) GetInt8() int64 {
	if len(s) == 0 {
		return 0
	}
	return int64(s[0])
}
`
)
