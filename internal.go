package echoswagger

import (
	"bytes"
	"html/template"
	"net/http"
	"reflect"
	"encoding/json"

	"github.com/labstack/echo/v4"
)

type ParamInType string

const (
	ParamInQuery    ParamInType = "query"
	ParamInHeader   ParamInType = "header"
	ParamInPath     ParamInType = "path"
	ParamInFormData ParamInType = "formData"
	ParamInBody     ParamInType = "body"
)

type UISetting struct {
	DetachSpec bool
	HideTop    bool
	CDN        string
}

type RawDefineDic struct {
	Schemas   map[string]RawDefineSchema
	Responses map[int]RawDefineReponse
}

type RawDefineSchema struct {
	Value  reflect.Value
	Schema *JSONSchema
}

type RawDefineReponse struct {
	Response *Response
}

func (r *Root) HTML(c echo.Context) error {
	return c.HTML(http.StatusOK, oauthRedirect)
}

func (r *Root) docHandler(docPath string) echo.HandlerFunc {
	t, err := template.New("swagger").Parse(swaggerHTMLTemplate)
	if err != nil {
		panic(err)
	}

	cdn := r.ui.CDN
	if cdn == "" {
		cdn = DefaultCDN
	}
	buf := new(bytes.Buffer)
	params := map[string]interface{}{
		"title": r.spec.Info.Title,
		"cdn":   cdn,
	}
	t.Execute(buf, params)

	return func(c echo.Context) error {
		cdn := r.ui.CDN
		if cdn == "" {
			cdn = DefaultCDN
		}
		buf := new(bytes.Buffer)
		params := map[string]interface{}{
			"title":    r.spec.Info.Title,
			"cdn":      cdn,
			"specName": SpecName,
		}
		if !r.ui.DetachSpec {
			spec, err := r.GetSpec(c, docPath)
			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			b, err := json.Marshal(spec)
			if err != nil {
				return c.String(http.StatusInternalServerError, err.Error())
			}
			params["spec"] = string(b)
			params["docPath"] = docPath
			params["hideTop"] = true
		} else {
			params["hideTop"] = r.ui.HideTop
		}
		if err := t.Execute(buf, params); err != nil {
			return c.String(http.StatusInternalServerError, err.Error())
		}
		return c.HTMLBlob(http.StatusOK, buf.Bytes())
	}
}

func (r *RawDefineDic) getKey(v reflect.Value) (bool, string) {
	name := v.Type().Name()

	if i, ok := v.Interface().(Name); ok {
		name = i.StructName()
	}

	if _, ok := r.Schemas[name]; ok {
		return true, name
	}

	return false, name
}

func (r *RawDefineDic) handleParamStruct(rt reflect.Value, in ParamInType, o *Operation) {
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Type().Field(i)
		if f.Type.Kind() == reflect.Struct && f.Anonymous {
			r.handleParamStruct(rt.Field(i), in, o)
		} else {
			name, _ := getFieldName(f, in)
			if name == "-" {
				continue
			}
			schema := r.genSchema(rt.Field(i))

			pm := &Parameter{
				Name:   name,
				In:     string(in),
				Schema: schema,
			}

			pm.Name = o.rename(pm.Name)
			o.Parameters = append(o.Parameters, pm)

			tags := getSwaggerTags(f)

			if _, ok := tags["required"]; ok {
				pm.Required = true
			}

			if schema.Ref == "" {
				handleSwaggerTags(schema, f, tags)
			}
		}
	}
}

func (r *routers) appendRoute(route *echo.Route) *api {
	opr := Operation{
		Responses: make(map[string]*Response),
	}
	a := api{
		route:     route,
		defs:      r.defs,
		operation: opr,
	}
	r.apis = append(r.apis, a)
	return &r.apis[len(r.apis)-1]
}

func (g *api) addParams(p interface{}, in ParamInType, name, desc string, required, nest bool) Api {
	if !isValidParam(reflect.TypeOf(p), nest, false) {
		panic("echoswagger: invalid " + string(in) + " param")
	}
	rt := indirectValue(p)
	st, sf := toSwaggerType(rt.Type())
	if st == "object" && sf == "object" {
		g.defs.handleParamStruct(rt, in, &g.operation)
	} else {
		name = g.operation.rename(name)
		schema := &JSONSchema{
			Type: JSONType(st),
		}
		pm := &Parameter{
			Name:        name,
			In:          string(in),
			Description: desc,
			Required:    required,
			Schema:      schema,
		}
		if st == "array" {
			schema.Items = g.defs.genSchema(rt)
		} else {
			schema.Format = sf
		}

		g.operation.Parameters = append(g.operation.Parameters, pm)
	}
	return g
}

func (g *api) addBodyParams(p interface{}, contentType, desc string, required bool) Api {
	if !isValidSchema(reflect.TypeOf(p), false) {
		panic("echoswagger: invalid body parameter")
	}

	if g.operation.RequestBody == nil {
		g.operation.RequestBody = &RequestBody{
			Description: desc,
			Required:    true,
			Content:     map[string]MediaType{},
		}
	}

	for key, _ := range g.operation.RequestBody.Content {
		if key == contentType {
			panic("echoswagger: multiple content type in request body are not allowed")
		}
	}

	rv := indirectValue(p)

	g.operation.RequestBody.Content[contentType] = MediaType{
		Schema: g.defs.genSchema(rv),
	}

	return g
}

func (o Operation) rename(s string) string {
	for _, p := range o.Parameters {
		if p.Name == s {
			return o.rename(s + "_")
		}
	}
	return s
}
