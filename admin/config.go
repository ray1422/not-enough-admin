package admin

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// SiteConfig is for the admin site config
type SiteConfig struct {
	DB          *gorm.DB
	models      []Model
	router      *gin.RouterGroup
	Middlewares []gin.HandlerFunc
}

// RegisterModel register orm to admin panel
func (c *SiteConfig) RegisterModel(model Model) {
	c.models = append(c.models, model)
}

// InitRouter regisyter GIN router
func (c SiteConfig) InitRouter(r *gin.RouterGroup) error {
	models := []string{}
	for _, v := range c.models {
		typeinfo := reflect.TypeOf(v.ORM).Elem()
		models = append(models, typeinfo.Name())
	}
	c.router = r
	r.GET("/", func(ctx *gin.Context) {
		ctx.HTML(http.StatusOK, "index.go.html", gin.H{
			"model_base_path": "",
			"model_name":      "",
			"base_url":        c.router.BasePath(),
			"models":          models,
		})
	})
	r.Static("static", "static/")
	for _, model := range c.models {
		info := reflect.TypeOf(model.ORM).Elem()

		handlersList := make([]gin.HandlerFunc, len(c.Middlewares))
		copy(handlersList, c.Middlewares)
		handlersList = append(handlersList, c.generateList(model))

		handlersUpsert := make([]gin.HandlerFunc, len(c.Middlewares))
		copy(handlersUpsert, c.Middlewares)
		handlersUpsert = append(handlersUpsert, c.generateUpsert(model))

		handlersRetrieve := make([]gin.HandlerFunc, len(c.Middlewares))
		copy(handlersRetrieve, c.Middlewares)
		handlersRetrieve = append(handlersRetrieve, c.generateRetrieve(model))

		r.GET(info.Name(), handlersList...)
		r.POST(info.Name(), handlersUpsert...)
		r.GET(info.Name()+"/:id", handlersRetrieve...)

		c.registerRel(model)
	}

	return nil
}

func (c SiteConfig) registerRel(model Model) (err error) {

	s, err := schema.Parse(model.ORM, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		return err
	}
	for _, field := range s.Fields {
		if field.FieldType.Kind() != reflect.Slice {
			continue
		}
		if s.Relationships.Relations[field.Name] == nil {
			continue
		}
		relation := s.Relationships.Relations[field.Name]
		relName := ""
		selfName := ""
		// selfName := ""
		for _, ref := range relation.References {
			if !ref.OwnPrimaryKey {
				relName = ref.ForeignKey.DBName
			} else {
				selfName = ref.ForeignKey.DBName
			}

		}
		tags := map[string]string{}
		tagsStrPairs := strings.Split(field.Tag.Get("gorm"), ",")
		for _, tagsStrPair := range tagsStrPairs {
			KsSplit := strings.Split(tagsStrPair, ":")
			if len(KsSplit) < 2 {
				tags[KsSplit[0]] = "__default__"
			} else {
				tags[KsSplit[0]] = KsSplit[1]
			}
		}
		preloads2 := []string{}
		for _, preload := range model.Preloads {
			spl := strings.Split(preload, ".")
			if len(spl) > 1 && spl[0] == field.Name {
				preloads2 = append(preloads2, strings.Join(spl[1:], "."))
			}
		}
		model2 := Model{
			ORM:      reflect.New(field.FieldType.Elem()).Interface(),
			Preloads: preloads2,
		}
		if len(s.PrimaryFields) == 0 {
			_ = fmt.Errorf("%s must have primary key for %s relation", s.Name, field.Name)
			return errors.New("primary key not found")
		}

		if relation.JoinTable != nil {
			// many2many relation
			joinTable := relation.JoinTable.Table

			listHandler := make([]gin.HandlerFunc, len(c.Middlewares))
			copy(listHandler, c.Middlewares)
			listHandler = append(listHandler, c.generateList(model2, joinTable, relName, selfName, model.ORM))
			c.router.GET(s.Name+"/:id/"+field.Name, listHandler...)
			// c.router.POST(s.Name+"/:id/"+field.Name, generateUpsert(model2, joinTable, relName, selfName, model.ORM))
		} else {
			// many2one
			listHandler := make([]gin.HandlerFunc, len(c.Middlewares))
			copy(listHandler, c.Middlewares)
			listHandler = append(listHandler, c.generateList(model2, selfName))
			c.router.GET(s.Name+"/:id/"+field.Name, listHandler...)
		}

	}

	return err
}
