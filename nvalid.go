package nvalid

import (
	"bytes"
	"io"
	"net/http"
	"net/url"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/muir/nject/nject"
	"github.com/muir/nject/nvelope"
	"github.com/pkg/errors"
)

// OpenAPI3Validator returns request and response validators that
// can drop into an nvelope-based http handler chain.  The first
// returned function should be placed just before or after the request
// decoder.  The second function should be as the parameter for
// nvelope.WithAPIEnforcer().
//
// OpenAPI3Validator's parameters are the location (file or URL) where
// the swagger can be found and an option specifier for openapi3filter:
// should it return MultiError or just a single error.
func OpenAPI3Validator(
	swaggerLocation string, // can be a URL or a filename(
	multiError bool,
) (
	func(r *http.Request, body nvelope.Body) nject.TerminalError,
	nvelope.APIEnforcerFunc,
	error,
) {
	if swaggerURL, err := url.Parse(swaggerLocation); err == nil {
		doc, err := openapi3.NewLoader().LoadFromURI(swaggerURL)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "load swagger from %s", swaggerLocation)
		}
		return OpenAPI3ValidatorFromParsed(doc, swaggerLocation, multiError)
	}
	doc, err := openapi3.NewLoader().LoadFromFile(swaggerLocation)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "load swagger file %s", swaggerLocation)
	}
	return OpenAPI3ValidatorFromParsed(doc, swaggerLocation, multiError)
}

// OpenAPI3ValidatorFromParsed returns request and response validators that
// can drop into an nvelope-based http handler chain.  The first
// returned function should be placed just before or after the request
// decoder.  The second function should be as the parameter for
// nvelope.WithAPIEnforcer().
//
// OpenAPI3Validator's parameters are the parsed API specification, the
// location (file or URL) where the swagger can be found; and an option
// specifier for openapi3filter: should it return MultiError or just a single error.
func OpenAPI3ValidatorFromParsed(
	doc *openapi3.T,
	swaggerLocation string,
	multiError bool,
) (
	func(r *http.Request, body nvelope.Body) nject.TerminalError,
	nvelope.APIEnforcerFunc,
	error,
) {
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "create router from swagger at %s", swaggerLocation)
	}
	request := func(r *http.Request, body nvelope.Body) nject.TerminalError {
		route, pathParams, err := router.FindRoute(r)
		if err != nil {
			return errors.Wrapf(err, "Find route for request to %s", r.URL)
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		err = openapi3filter.ValidateRequest(r.Context(), &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: pathParams,
			Route:      route,
			Options: &openapi3filter.Options{
				MultiError: multiError,
			},
		})
		return err
	}
	response := func(httpCode int, enc []byte, header http.Header, r *http.Request) error {
		route, pathParams, err := router.FindRoute(r)
		if err != nil {
			return errors.Wrapf(err, "Find route for request to %s", r.URL)
		}
		err = openapi3filter.ValidateResponse(r.Context(), &openapi3filter.ResponseValidationInput{
			RequestValidationInput: &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
				Options: &openapi3filter.Options{
					MultiError: multiError,
				},
			},
			Status: httpCode,
			Header: header,
			Body:   io.NopCloser(bytes.NewReader(enc)),
			Options: &openapi3filter.Options{
				MultiError: multiError,
			},
		})
		return err
	}
	return request, response, nil
}
