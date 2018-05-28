package util

import (
	"fmt"
	"net/http"
)

func FprintDescriptionAndCodeAsJSON(responseWriter http.ResponseWriter, code string, description string, a ...interface{}) {
	fmt.Fprintf(responseWriter, DescriptionAndCodeAsJSON(code, description, a...))
}

func DescriptionAndCodeAsJSON(code string, description string, a ...interface{}) string {
	return fmt.Sprintf(`{"description":"%v","code":%v}`, fmt.Sprintf(description, a...), code)
}

func FprintDescriptionAsJSON(responseWriter http.ResponseWriter, description string, a ...interface{}) {
	fmt.Fprintf(responseWriter, DescriptionAsJSON(description, a...))
}

func DescriptionAsJSON(description string, a ...interface{}) string {
	return fmt.Sprintf(`{"description":"%v"}`, fmt.Sprintf(description, a...))
}
