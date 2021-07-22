package echoswagger

import (
	"reflect"
	"strconv"
	"strings"
)

// getTag reports is a tag exists and it's content
// search tagName in all tags when index = -1
func getTag(field reflect.StructField, tagName string, index int) (bool, string) {
	t := field.Tag.Get(tagName)
	s := strings.Split(t, ",")

	if len(s) < index+1 {
		return false, ""
	}

	return true, strings.TrimSpace(s[index])
}

func getSwaggerTags(field reflect.StructField) map[string]string {
	t := field.Tag.Get("swagger")
	r := make(map[string]string)
	for _, v := range strings.Split(t, ";") {
		if v == "" {
			continue
		}
		split := strings.Split(v, "=")
		if len(split) > 1 {
			r[split[0]] = strings.Join(split[1:], "=")
		} else {
			r[v] = ""
		}
	}

	t = field.Tag.Get("validate")
	child := false
	keys := false
	for _, v := range strings.Split(t, ",") {
		if v == "" {
			continue
		}
		split := strings.Split(v, "=")
		if len(split) > 1 {
			key := split[0]

			if child {
				key = "child_" + key
			}
			if keys {
				key = "keys_" + key
			}

			r[key] = strings.Join(split[1:], "=")
		} else {
			if split[0] == "dive" {
				child = true
				continue
			}

			if split[0] == "keys" {
				keys = true
				continue
			}

			if split[0] == "endkeys" {
				keys = false
				continue
			}
			r[v] = ""
		}
	}

	return r
}

func getFieldName(f reflect.StructField, in ParamInType) (string, bool) {
	if f.Tag.Get("swagger") == "-" {
		return "-", false
	}

	var name string
	switch in {
	case ParamInQuery:
		name = f.Tag.Get("query")
	case ParamInFormData:
		name = f.Tag.Get("form")
	case ParamInBody, ParamInHeader, ParamInPath:
		_, name = getTag(f, "json", 0)
	}
	if name != "" {
		return name, true
	} else {
		return f.Name, false
	}
}

func handleSwaggerTags(propSchema *JSONSchema, f reflect.StructField, tags map[string]string) {
	if t, ok := tags["desc"]; ok {
		propSchema.Description = t
	}
	if t, ok := tags["min"]; ok {
		if m, err := strconv.ParseFloat(t, 64); err == nil {
			intvalue := int(m)
			if propSchema.Type == "array" {
				propSchema.MinItems = &intvalue
			} else if propSchema.Type == "object" {
				propSchema.MinProperties = &intvalue
			} else if propSchema.Type == "string" {
				propSchema.MinLength = &intvalue
			} else {
				propSchema.Minimum = &m
			}
		}
	}
	if t, ok := tags["max"]; ok {
		if m, err := strconv.ParseFloat(t, 64); err == nil {
			intvalue := int(m)
			if propSchema.Type == "array" {
				propSchema.MaxItems = &intvalue
			} else if propSchema.Type == "object" {
				propSchema.MaxProperties = &intvalue
			} else if propSchema.Type == "string" {
				propSchema.MaxLength = &intvalue
			} else {
				propSchema.Maximum = &m
			}
		}
	}
	if t, ok := tags["minLen"]; ok {
		if m, err := strconv.Atoi(t); err == nil {
			propSchema.MinLength = &m
		}
	}
	if t, ok := tags["maxLen"]; ok {
		if m, err := strconv.Atoi(t); err == nil {
			propSchema.MaxLength = &m
		}
	}

	if t, ok := tags["child_max"]; ok {
		if m, err := strconv.ParseInt(t, 10, 64); err == nil {
			i := int(m)
			propSchema.AdditionalProperties.MaxLength = &i
		}
	}

	if _, ok := tags["readOnly"]; ok {
		propSchema.ReadOnly = true
	}
	if _, ok := tags["nullable"]; ok {
		propSchema.Nullable = true
	}
	if _, ok := tags["omitempty"]; ok {
		propSchema.Nullable = true
	}

	if tag, ok := tags["type"]; ok {
		propSchema.Type = JSONType(tag)
		propSchema.Ref = ""
	}

	if tag, ok := tags["format"]; ok {
		propSchema.Format = tag
	}

	convert := converter(f.Type)
	if t, ok := tags["enum"]; ok {
		enums := strings.Split(t, ",")
		var es []interface{}
		for _, s := range enums {
			v, err := convert(s)
			if err != nil {
				continue
			}
			es = append(es, v)
		}
		propSchema.Enum = es
	}
	if t, ok := tags["default"]; ok {
		v, err := convert(t)
		if err == nil {
			propSchema.DefaultValue = v
		}
	}

	if tag, ok := tags["example"]; ok {
		v, err := convert(tag)
		if err != nil {
			panic(err)
		}
		propSchema.Example = v
	}

	// Move part of tags in Schema to Items
	if propSchema.Type == "array" {
		items := propSchema.Items.latest()
		items.Minimum = propSchema.Minimum
		items.Maximum = propSchema.Maximum
		items.MinLength = propSchema.MinLength
		items.MaxLength = propSchema.MaxLength
		items.Enum = propSchema.Enum
		items.DefaultValue = propSchema.DefaultValue
		propSchema.Minimum = nil
		propSchema.Maximum = nil
		propSchema.MinLength = nil
		propSchema.MaxLength = nil
		propSchema.Enum = nil
		propSchema.DefaultValue = nil
	}
}

func (s *JSONSchema) latest() *JSONSchema {
	if s.Items != nil {
		return s.Items.latest()
	}
	return s
}
