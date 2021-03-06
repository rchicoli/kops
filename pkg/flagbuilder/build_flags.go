/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package flagbuilder

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kops/upup/pkg/fi/utils"
)

// BuildFlags builds flag arguments based on "flag" tags on the structure
func BuildFlags(options interface{}) (string, error) {
	var flags []string

	walker := func(path string, field *reflect.StructField, val reflect.Value) error {
		if field == nil {
			glog.V(8).Infof("ignoring non-field: %s", path)
			return nil
		}
		tag := field.Tag.Get("flag")
		if tag == "" {
			glog.V(4).Infof("not writing field with no flag tag: %s", path)
			// We want to descend - it could be a structure containing flags
			return nil
		}
		if tag == "-" {
			glog.V(4).Infof("skipping field with %q flag tag: %s", tag, path)
			return utils.SkipReflection
		}
		flagName := tag

		// If the "unset" value is not empty string, by setting this tag we avoid passing spurious flag values
		flagEmpty := field.Tag.Get("flag-empty")

		// We do have to do this, even though the recursive walk will do it for us
		// because when we descend we won't have `field` set
		if val.Kind() == reflect.Ptr {
			if val.IsNil() {
				return nil
			}
			val = val.Elem()
		}

		if val.Kind() == reflect.Map {
			if val.IsNil() {
				return nil
			}
			// We handle a map[string]string like --node-labels=k1=v1,k2=v2 etc
			// As we need more formats we can add additional spec to the flags tag
			if stringStringMap, ok := val.Interface().(map[string]string); ok {
				var args []string
				for k, v := range stringStringMap {
					arg := fmt.Sprintf("%s=%s", k, v)
					args = append(args, arg)
				}
				if len(args) != 0 {
					flag := fmt.Sprintf("--%s=%s", flagName, strings.Join(args, ","))
					flags = append(flags, flag)
				}
				return utils.SkipReflection
			} else {
				return fmt.Errorf("BuildFlags of value type not handled: %T %s=%v", val.Interface(), path, val.Interface())
			}
		}

		if val.Kind() == reflect.Slice {
			if val.IsNil() {
				return nil
			}
			// We handle a []string like --admission-control=v1,v2 etc
			if stringSlice, ok := val.Interface().([]string); ok {
				if len(stringSlice) != 0 {
					flag := fmt.Sprintf("--%s=%s", flagName, strings.Join(stringSlice, ","))
					flags = append(flags, flag)
				}
				return utils.SkipReflection
			} else {
				return fmt.Errorf("BuildFlags of value type not handled: %T %s=%v", val.Interface(), path, val.Interface())
			}
		}

		var flag string
		switch v := val.Interface().(type) {
		case string:
			vString := fmt.Sprintf("%v", v)
			if vString != "" && vString != flagEmpty {
				flag = fmt.Sprintf("--%s=%s", flagName, vString)
			}

		case bool, int, int32, int64, float32, float64:
			vString := fmt.Sprintf("%v", v)
			if vString != flagEmpty {
				flag = fmt.Sprintf("--%s=%s", flagName, vString)
			}

		case metav1.Duration:
			vString := v.Duration.String()

			// See https://github.com/kubernetes/kubernetes/issues/40783
			// Go renders a time.Duration to `0` in <= 1.6, and `0s` in >= 1.7
			// We force it to be `0s`, regardless of value
			if vString == "0" {
				vString = "0s"
			}

			if vString != flagEmpty {
				flag = fmt.Sprintf("--%s=%s", flagName, vString)
			}

		default:
			return fmt.Errorf("BuildFlags of value type not handled: %T %s=%v", v, path, v)
		}
		if flag != "" {
			flags = append(flags, flag)
		}
		// Nothing more to do here
		return utils.SkipReflection
	}
	err := utils.ReflectRecursive(reflect.ValueOf(options), walker)
	if err != nil {
		return "", err
	}
	// Sort so that the order is stable across runs
	sort.Strings(flags)

	return strings.Join(flags, " "), nil
}
