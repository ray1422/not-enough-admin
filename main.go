package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"reflect"
	"strconv"

	"github.com/Masterminds/sprig/v3"
	"github.com/gin-gonic/gin"
	"github.com/ray1422/not-enough-admin/admin"
	"github.com/ray1422/not-enough-admin/db"
	"github.com/ray1422/not-enough-admin/util"
	"gorm.io/gorm"
)

var (
	dsn = "host=localhost user=postgres dbname=" + util.Getenv("DB_NAME", "myadmin") + " port=5432 sslmode=disable TimeZone=Asia/Taipei" // TODO read from env var
)

// Foo Foo
type Foo struct {
	gorm.Model
	Asdf   string
	Qwerty int
	WWW    []Goo `gorm:"many2many:foo_goos;constraint:OnDelete:CASCADE"`
	Goos   []Goo

	MyBool bool
}

// Goo Goo
type Goo struct {
	gorm.Model
	FooID sql.NullInt64
	Foo   *Foo `gorm:"constraint:OnDelete:CASCADE;"`
	A     int
}

func main() {

	db.GormDB().AutoMigrate(&Foo{})
	db.GormDB().AutoMigrate(&Goo{})
	db.GormDB().Unscoped().Where("1 = 1").Delete(&Goo{})
	db.GormDB().Unscoped().Where("1 = 1").Delete(&Foo{})
	g := []*Goo{
		{A: 1},
		{A: 2},
	}

	for _, v := range g {
		db.GormDB().Create(v)
	}
	for i := 0; i < 15; i++ {
		db.GormDB().Create(&Foo{
			Asdf:   "Foo " + strconv.Itoa(i),
			Qwerty: i,
			Goos: []Goo{
				{A: rand.Int() % 20},
				{A: rand.Int() % 20},
			},
			WWW: []Goo{
				{A: rand.Int()%20 + 300},
				{A: rand.Int()%20 + 300},
			},
		})
	}

	r := gin.Default()

	r.SetFuncMap(sprig.FuncMap())
	r.SetHTMLTemplate(admin.Tmpls())
	// r.LoadHTMLGlob("admin/views/*")
	// r.LoadHTMLGlob("admin/views/templates/*")
	conf := admin.SiteConfig{
		Middlewares: []gin.HandlerFunc{
			func(ctx *gin.Context) {
				fmt.Println("ctx", ctx)
			},
		},
	}

	conf.RegisterModel(admin.Model{
		ORM: &Foo{},
		Filters: []admin.Pair[string, reflect.Type]{
			{First: "id", Second: reflect.TypeOf(int(1))},
			{First: "asdf", Second: reflect.TypeOf(string(""))},
		},
		Preloads: []string{"Goos"},
	})
	conf.RegisterModel(admin.Model{
		ORM: &Goo{},
		Filters: []admin.Pair[string, reflect.Type]{
			{First: "id", Second: reflect.TypeOf(int(1))},
		},
	})
	conf.InitRouter(r.Group("asdf"))
	r.Run()

}
