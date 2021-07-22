package echoswagger

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/labstack/echo/v4"
)

const (
	SchemaPrefix   = "#/components/schemas/"
	ResponsePrefix = "#/components/responses/"
	SwaggerVersion = "3.0.1"
	SpecName       = "swagger.json"
)

func (r *Root) specHandler(docPath string) echo.HandlerFunc {
	return func(c echo.Context) error {
		spec, err := r.GetSpec(c, docPath)
		if err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.JSON(http.StatusOK, spec)
	}
}

// Generate swagger spec data, without host & basePath info
func (r *Root) GetSpec(c echo.Context, docPath string) (Swagger, error) {
	r.once.Do(func() {
		r.err = r.genSpec(c)
	})
	if r.err != nil {
		return Swagger{}, r.err
	}
	return *r.spec, nil
}

func (r *Root) genSpec(c echo.Context) error {
	r.spec.Openapi = SwaggerVersion
	r.spec.Paths = make(map[string]interface{})

	for i := range r.groups {
		group := &r.groups[i]

		found := false
		for _, tag := range r.spec.Tags {
			if tag.Name == group.tag.Name {
				found = true
				break
			}
		}
		if !found {
			r.spec.Tags = append(r.spec.Tags, &group.tag)
		}

		for j := range group.apis {
			a := &group.apis[j]
			if err := a.operation.addSecurity(r.spec.Components.SecuritySchemes, group.security); err != nil {
				return err
			}
			if err := r.transfer(a); err != nil {
				return err
			}
		}
	}

	for i := range r.apis {
		if err := r.transfer(&r.apis[i]); err != nil {
			return err
		}
	}

	for k, v := range r.defs.Schemas {
		r.spec.Components.Schemas[k] = v.Schema
	}
	for k, v := range r.defs.Responses {
		r.spec.Components.Responses[fmt.Sprintf("%d", k)] = v.Response
	}
	return nil
}

func (r *Root) transfer(a *api) error {
	if err := a.operation.addSecurity(r.spec.Components.SecuritySchemes, a.security); err != nil {
		return err
	}

	path := toSwaggerPath(a.route.Path)
	if len(a.operation.Responses) == 0 {
		a.operation.Responses["default"] = &Response{
			Description: "successful operation",
		}
	}

	if p, ok := r.spec.Paths[path]; ok {
		p.(*Path).oprationAssign(a.route.Method, &a.operation)
	} else {
		p := &Path{}
		p.oprationAssign(a.route.Method, &a.operation)
		r.spec.Paths[path] = p
	}
	return nil
}

func (p *Path) oprationAssign(method string, operation *Operation) {
	switch method {
	case echo.GET:
		p.Get = operation
	case echo.POST:
		p.Post = operation
	case echo.PUT:
		p.Put = operation
	case echo.DELETE:
		p.Delete = operation
	case echo.OPTIONS:
		p.Options = operation
	case echo.HEAD:
		p.Head = operation
	case echo.PATCH:
		p.Patch = operation
	}
}

func (r *Root) cleanUp() {
	r.echo = nil
	r.groups = nil
	r.apis = nil
	r.defs = nil
}

// addDefinition adds definition specification and returns
// key of RawDefineDic
func (r *RawDefineDic) addDefinition(v reflect.Value) string {
	if i, ok := v.Interface().(Shim); ok {
		v = reflect.ValueOf(i.Shim())
	}

	exist, key := r.getKey(v)
	if exist {
		return key
	}

	var schema = &JSONSchema{}

	if v.Kind() == reflect.String {
		schema.Type = "string"
		if i, ok := v.Interface().(Enum); ok {
			schema.Enum = i.EnumValues()
		} else {
			panic("not enum")
		}
	} else {
		schema.Type = "object"
		schema.Properties = make(map[string]*JSONSchema)
		r.handleStruct(v, schema)
	}

	r.Schemas[key] = RawDefineSchema{
		Value:  v,
		Schema: schema,
	}

	if i, ok := v.Interface().(Description); ok {
		schema.Description = i.Description()
	}

	return key
}

// handleStruct handles fields of a struct
func (r *RawDefineDic) handleStruct(v reflect.Value, schema *JSONSchema) {
	for i := 0; i < v.NumField(); i++ {
		f := v.Type().Field(i)
		name, hasTag := getFieldName(f, ParamInBody)
		if name == "-" {
			continue
		}
		if f.Type.Kind() == reflect.Struct && f.Anonymous && !hasTag {
			r.handleStruct(v.Field(i), schema)
			continue
		}

		var sp *JSONSchema
		if strings.HasPrefix(f.Tag.Get("swagger"), "type") {
			sp = &JSONSchema{}
		} else {
			sp = r.genSchema(v.Field(i))
		}

		tags := getSwaggerTags(f)

		if _, ok := tags["required"]; ok {
			schema.Required = append(schema.Required, name)
		}

		if sp.Ref == "" {
			handleSwaggerTags(sp, f, tags)
		}

		schema.Properties[name] = sp
	}
}

type Enum interface {
	EnumValues() []interface{}
}

type Description interface {
	Description() string
}

type Name interface {
	StructName() string
}

type Shim interface {
	Shim() interface{}
}
