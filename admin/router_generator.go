package admin

import (
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/badgerodon/collections"
	"github.com/gin-gonic/gin"
	"github.com/pilagod/gorm-cursor-paginator/v2/paginator"
	"github.com/ray1422/not-enough-admin/db"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type manyWrapper struct {
	Name  string
	Value any
}

func clear(v interface{}) {
	p := reflect.ValueOf(v).Elem()
	p.Set(reflect.Zero(p.Type()))
}

// transpose copied from GitHub. Hope it's bugless.
func transpose(slice [][]any) [][]any {
	if len(slice) == 0 {
		return nil
	}
	xl := len(slice[0])
	yl := len(slice)
	result := make([][]any, xl)
	for i := range result {
		result[i] = make([]any, yl)
	}
	for i := 0; i < xl; i++ {
		for j := 0; j < yl; j++ {
			result[i][j] = slice[j][i]
		}
	}
	return result
}

func createPaginator(
	cursor paginator.Cursor,
) *paginator.Paginator {
	opts := []paginator.Option{
		&paginator.Config{
			Keys:  []string{"ID", "CreatedAt"},
			Limit: 10,
			Order: paginator.DESC,
		},
	}

	if cursor.After != nil {
		opts = append(opts, paginator.WithAfter(*cursor.After))
	}
	if cursor.Before != nil {
		opts = append(opts, paginator.WithBefore(*cursor.Before))
	}
	return paginator.New(opts...)
}

func keyValPairExt(obj interface{}, keyValPair *[]Pair[string, any], prefix string, preloads []string) {
	info := reflect.TypeOf(obj)
	val := reflect.ValueOf(obj)
	if info.Kind() == reflect.Ptr && !val.IsZero() {
		info = info.Elem()
		val = val.Elem()
	}

	for i := 0; i < info.NumField(); i++ {
		field := info.Field(i)
		if !field.IsExported() {
			log.Println(info.Name() + "." + field.Name + " is not exported.")
			continue
		}
		if field.Type.AssignableTo(reflect.TypeOf(time.Time{})) {
			*keyValPair = append(*keyValPair, Pair[string, any]{First: prefix + "." + field.Name,
				Second: val.FieldByName(field.Name).Interface().(time.Time).Format(time.RFC3339)})
			continue
		}

		if field.Type.Kind() == reflect.Struct {
			keyValPairExt(val.FieldByName(field.Name).Interface(), keyValPair, prefix+"."+field.Name, preloads)
			continue
		}
		if field.Type.Kind() == reflect.Ptr &&
			reflect.TypeOf(reflect.ValueOf(val.FieldByName(field.Name).Interface())).Kind() == reflect.Struct {
			if val.FieldByName(field.Name).IsZero() {
				s := prefix + field.Name
				s = s[1:]
				load := false
				for _, v := range preloads {
					if v[:len(s)] == s {
						load = true
						break
					}
				}
				if load {
					keyValPairExt(reflect.Indirect(val.FieldByName(field.Name)), keyValPair, prefix+"."+field.Name, preloads)
				} else {
					*keyValPair = append(*keyValPair, Pair[string, any]{First: prefix + "." + field.Name, Second: "null"})
				}
				continue
			}
			keyValPairExt(val.FieldByName(field.Name).Elem().Interface(), keyValPair, prefix+"."+field.Name, preloads)
			continue
		}
		if field.Type.Kind() == reflect.Slice {
			*keyValPair = append(*keyValPair, Pair[string, any]{First: prefix + "." + field.Name, Second: manyWrapper{
				Name:  field.Name,
				Value: val.FieldByName(field.Name).Interface(),
			}})
			continue
		}
		*keyValPair = append(*keyValPair, Pair[string, any]{First: prefix + "." + field.Name, Second: val.FieldByName(field.Name).Interface()})
	}
}

func (c SiteConfig) generateRetrieve(model Model) func(c *gin.Context) {

	modelInfo, err := schema.Parse(model.ORM, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		panic(err)
	}
	modelBasePath := c.router.BasePath() + "/" + modelInfo.Name
	baseURL := c.router.BasePath()
	models := []string{}
	for _, v := range c.models {
		models = append(models, reflect.TypeOf(v.ORM).Elem().Name())
	}
	// TODO has many, many2many rel
	return func(c *gin.Context) {
		itemURL := *c.Request.URL
		itemURL.RawQuery = ""
		clear(model.ORM)
		idStr := c.Param("id")
		id, err := strconv.Atoi(idStr)
		_ = id
		if err != nil {
			c.Status(400)
			return
		}
		tx := db.GormDB()
		for _, preload := range model.Preloads {
			tx = tx.Preload(preload)
		}

		typeInfo := reflect.ValueOf(model.ORM).Elem().Type()
		ptr := reflect.New(typeInfo)
		err = tx.Take(ptr.Interface(), id).Error
		if err != nil {
			log.Println(err)
			c.Status(404)
			return
		}
		kvPair := []Pair[string, any]{}
		keyValPairExt(ptr.Interface(), &kvPair, "", model.Preloads)
		ret := gin.H{
			"models":          models,
			"model_base_path": modelBasePath,
			"model_name":      typeInfo.Name(),
			"base_url":        baseURL,
			"items":           gin.H{},
			"item_url":        itemURL.String(),
		}
		for _, v := range kvPair {
			ret["items"].(gin.H)[v.First] = gin.H{
				"value": v.Second,
			}
			switch reflect.TypeOf(v.Second).Kind() {
			case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
				ret["items"].(gin.H)[v.First].(gin.H)["type"] = "number"
			case reflect.Bool:
				ret["items"].(gin.H)[v.First].(gin.H)["type"] = "checkbox"
			case reflect.Struct:
				if u, ok := v.Second.(manyWrapper); ok {
					ret["items"].(gin.H)[v.First].(gin.H)["type"] = "many"
					ret["items"].(gin.H)[v.First].(gin.H)["value"] = u.Name
				}

			default:
				ret["items"].(gin.H)[v.First].(gin.H)["type"] = "text"

			}
		}

		c.HTML(http.StatusOK, "retrieve.go.html", ret)
	}
}

// generateList opts := {joinTableName, relName, selfName, relModelDB}
func (c SiteConfig) generateList(model Model, opts ...interface{}) func(*gin.Context) {
	models := []string{}
	for _, v := range c.models {
		typeinfo := reflect.TypeOf(v.ORM).Elem()
		models = append(models, typeinfo.Name())
	}

	var joinTableName, selfName, relName, relTable, relPK string
	_ = relTable
	_ = relPK
	if len(opts) == 4 {
		joinTableName = opts[0].(string)
		relName = opts[1].(string)
		selfName = opts[2].(string)
		relModelDB, err := schema.Parse(opts[3], &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			panic(err)
		}
		fmt.Println("relation with many2many:", joinTableName, selfName, relModelDB.Table, relModelDB.PrimaryFieldDBNames)
		relTable = relModelDB.Table
		relPK = relModelDB.PrimaryFieldDBNames[0]
	} else if len(opts) == 1 {
		// many2one
		selfName = opts[0].(string)

	}
	modelInfo, err := schema.Parse(model.ORM, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		panic(err)
	}

	modelBasePath := c.router.BasePath() + "/" + modelInfo.Name
	baseURL := c.router.BasePath()
	return func(c *gin.Context) {

		typeInfo := reflect.TypeOf(model.ORM).Elem()
		filters := []Pair[string, reflect.Type]{}
		clear(model.ORM)
		curAfterStr := c.Query("cur_after")
		curBeforeStr := c.Query("cur_before")
		cursor := paginator.Cursor{}
		if curAfterStr != "" {
			cursor.After = &curAfterStr
		} else if curBeforeStr != "" {
			cursor.Before = &curBeforeStr
		}
		for _, filter := range model.Filters {
			if c.Query(filter.First) != "" {
				filters = append(filters, filter)
			}
		}
		tx := db.GormDB().Model(model.ORM)
		if len(opts) == 4 {
			tx = tx.Joins(`INNER JOIN "`+joinTableName+`" ON "`+joinTableName+`"."`+relName+`" = "`+modelInfo.Table+`"."`+modelInfo.PrimaryFieldDBNames[0]+`"`).
				Where(`"`+joinTableName+`"."`+selfName+`"`, c.Param("id"))
		} else if len(opts) == 1 {
			tx = tx.Where(selfName, c.Param("id"))
		}
		for _, preload := range model.Preloads {
			tx = tx.Preload(preload)
		}
		for _, filter := range filters {
			tx = tx.Where(filter.First, c.Query(filter.First))
		}

		slice := reflect.MakeSlice(reflect.SliceOf(typeInfo), 0, 10)
		recordPtr := reflect.New(slice.Type())
		recordPtr.Elem().Set(slice)

		p := createPaginator(cursor)
		_, cursor, _ = p.Paginate(tx, recordPtr.Interface())

		// get Fields

		rets := map[string][]any{}
		records := recordPtr.Elem()
		fields := []string{}
		for i := 0; i < records.Len(); i++ {
			items := []Pair[string, any]{}
			keyValPairExt(records.Index(i).Interface(), &items, "", model.Preloads)

			if i == 0 {
				for _, v := range items {
					fields = append(fields, v.First)
				}
			}
			for _, item := range items {
				rets[item.First] = append(rets[item.First], item.Second)
			}
		}

		items := [][]any{}
		for _, field := range fields {
			v := rets[field]
			items = append(items, make([]any, 0))
			for _, u := range v {
				items[len(items)-1] = append(items[len(items)-1], u)
			}
		}

		items = transpose(items)
		url := c.Request.URL
		url.RawQuery = ""
		urlNext := *c.Request.URL
		if cursor.After != nil {
			v := urlNext.Query()
			v.Set("cur_after", *cursor.After)
			urlNext.RawQuery = v.Encode()

		}

		urlPrev := *c.Request.URL
		if cursor.Before != nil {
			v := urlPrev.Query()
			v.Set("cur_before", *cursor.Before)
			urlPrev.RawQuery = v.Encode()
		}

		c.HTML(http.StatusOK, "list.go.html", gin.H{
			"model_base_path": modelBasePath,
			"model_name":      typeInfo.Name(),
			"models":          models,
			"fields":          fields,
			"filters":         model.Filters,
			"items":           items,
			"next":            cursor.After,
			"next_url":        urlNext.String(),
			"prev":            cursor.Before,
			"prev_url":        urlPrev.String(),
			"base_url":        baseURL,
		})
	}
}

func (c SiteConfig) generateUpsert(model Model, opts ...interface{}) func(*gin.Context) {
	modelInfo, err := schema.Parse(model.ORM, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		panic(err)
	}
	modelBasePath := c.router.BasePath() + "/" + modelInfo.Name
	return func(c *gin.Context) {
		typeInfo := reflect.ValueOf(model.ORM).Elem().Type()
		ptr := reflect.New(typeInfo)
		// check model.ID must be int or not set
		_, err := strconv.Atoi(c.DefaultPostForm("Model.ID", "0"))

		if err != nil {
			c.Status(401)
			return
		}
		preloads := map[string]bool{}
		for _, v := range model.Preloads {
			preloads["."+v] = true
		}

		stk := collections.NewStack[Pair[string, *reflect.Value]]()
		initPush := ptr.Elem()
		stk.Push(Pair[string, *reflect.Value]{"", &initPush})
		for stk.Size() > 0 {
			top, _ := stk.Pop()
			fieldNameWithPrefix := top.First

			if top.Second.Kind() == reflect.Pointer && preloads[fieldNameWithPrefix] {
				top.Second.SetPointer(reflect.New(top.Second.Type()).UnsafePointer())
				fieldVal := top.Second.Elem()
				stk.Push(Pair[string, *reflect.Value]{fieldNameWithPrefix, &fieldVal})
				continue
			}
			if top.Second.Kind() == reflect.Struct && top.Second.Type() != reflect.TypeOf(time.Time{}) {
				for i := 0; i < top.Second.NumField(); i++ {
					if !top.Second.Type().Field(i).IsExported() {
						continue
					}
					field := top.Second.Field(i)
					stk.Push(Pair[string, *reflect.Value]{top.First + "." + top.Second.Type().Field(i).Name, &field})
				}
				continue
			}

			if val := c.PostForm(fieldNameWithPrefix); val != "" {

				switch top.Second.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
					reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					if v, err := strconv.Atoi(val); err == nil {
						if top.Second.CanInt() {
							top.Second.SetInt(int64(v))
						} else if top.Second.CanUint() {
							top.Second.SetUint(uint64(v))
						} else {
							log.Println(fieldNameWithPrefix, "can't be set to (u)int!")
						}
					}
				case reflect.Float32, reflect.Float64:
					if v, err := strconv.ParseFloat(val, 64); err == nil {
						if top.Second.CanFloat() {
							top.Second.SetFloat(v)
						} else {
							log.Println(fieldNameWithPrefix, "can't be set to float!")
						}
					}

				case reflect.Struct:
					if top.Second.Type() == reflect.TypeOf(time.Time{}) {
						if v, err := time.Parse(time.RFC3339, val); err == nil {
							timeVal := reflect.ValueOf(v)
							top.Second.Set(timeVal)

						} else {
							fmt.Println(err)
						}
					}
				case reflect.Bool:
					top.Second.SetBool(strings.ToLower(val) == "true")
				case reflect.String:
					top.Second.SetString(val)
				}
			}
		}
		save := ptr.Interface()
		tx := db.GormDB().Session(&gorm.Session{FullSaveAssociations: true}).
			Save(save).
			Updates(save)
		if tx.Error != nil {
			c.JSON(500, tx.Error)
		}

		pkReflect := ptr.Elem().FieldByName(modelInfo.PrimaryFields[0].Name)

		pk := fmt.Sprint(pkReflect.Interface())
		c.HTML(201, "redirect.go.html", gin.H{
			"redirect": modelBasePath + "/" + pk + "?save_time=" + fmt.Sprint(time.Now().UnixMicro()),
		})
	}
}
