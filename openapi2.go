package nvalid

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/ghodss/yaml"
	"github.com/muir/nject/nject"
	"github.com/muir/nject/nvelope"
	"github.com/pkg/errors"
)

// OpenAPI2Validator returns request and response validators that
// can drop into an nvelope-based http handler chain.  The first
// returned function should be placed just before or after the request
// decoder.  The second function should be as the parameter for
// nvelope.WithAPIEnforcer().
//
// OpenAPI2Validator's parameters are the filename where the JSON or
// YAML swagger can be found and an option specifier for openapi3filter:
// should it return MultiError or just a single error.
func OpenAPI2Validator(
	swaggerFile string,
	multiError bool,
) (
	func(r *http.Request, body nvelope.Body) nject.TerminalError,
	nvelope.APIEnforcerFunc,
	error,
) {
	input, err := os.ReadFile(swaggerFile)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "read %s", swaggerFile)
	}

	var v2Doc openapi2.T
	switch strings.ToLower(filepath.Ext(swaggerFile)) {
	case "json":
		err = json.Unmarshal(input, &v2Doc)
	case "yml", "yaml":
		err = yaml.Unmarshal(input, &v2Doc)
	default:
		return nil, nil, errors.Errorf("Cannot determine (based on extention) if %s is YAML or JSON", swaggerFile)
	}

	v3Doc, err := openapi2conv.ToV3(&v2Doc)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "convert %s to OpenAPI v3", swaggerFile)
	}

	return OpenAPI3ValidatorFromParsed(v3Doc, swaggerFile, multiError)
}
