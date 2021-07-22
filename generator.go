package echoswagger

import (
	"reflect"
)

func (r *RawDefineDic) genSchema(v reflect.Value) *JSONSchema {
	if !v.IsValid() {
		return nil
	}

	nullable := false
	if v.Kind() == reflect.Ptr {
		nullable = true
	}

	v = indirect(v)
	st, sf := toSwaggerType(v.Type())
	schema := &JSONSchema{
		Nullable: nullable,
	}

	if st == "array" {
		schema.Type = JSONType(st)
		if v.Len() == 0 {
			v = reflect.MakeSlice(v.Type(), 1, 1)
		}
		schema.Items = r.genSchema(v.Index(0))
	} else if st == "object" && sf == "map" {
		schema.Type = JSONType(st)
		if v.Len() == 0 {
			v = reflect.New(v.Type().Elem())
		} else {
			v = v.MapIndex(v.MapKeys()[0])
		}
		schema.AdditionalProperties = r.genSchema(v)
	} else if st == "object" {
		key := r.addDefinition(v)
		schema = &JSONSchema{
			Ref: SchemaPrefix + key,
		}
	} else {
		schema.Type = JSONType(st)
		schema.Format = sf
		zv := reflect.Zero(v.Type())
		if v.CanInterface() && zv.CanInterface() && v.Interface() != zv.Interface() {
			schema.Example = v.Interface()
		}
	}
	return schema
}
