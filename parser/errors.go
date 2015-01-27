package parser

import "fmt"

func UnexpectedHeaderLineError(fpath, giv string) error {
	return fmt.Errorf("File '%s' has an unexpected header line: '%s', expected format 'Header-Key: Value'", fpath, giv)
}

func UnexpectedResponseLineError(fpath, line string) error {
	return fmt.Errorf("Parser encountered a 'then' file '%s' with an unexpected first line: %s, expected format '<HTTP Status Code> <Status Text>'", fpath, line)
}

func UnexpectedResponseLineCodeError(fpath, giv string, err error) error {
	return fmt.Errorf("Parser encountered a 'then' file '%s' with an unexpected status code: %s, expected a number. (%s)", fpath, giv, err)
}

func UnexpectedRequestLineError(fpath, line string) error {
	return fmt.Errorf("Parser encountered a 'when' file '%s' with an unexpected first line: %s, expected format '<HTTP method> <path>'", fpath, line)
}

func UnexpectedRequestLineMethodError(fpath, giv string) error {
	return fmt.Errorf("Parser encountered a 'when' file '%s' with an unexpected HTTP Method in the first line: '%s', expected one of: %s", fpath, giv, ValidHTTPMethods)
}

func UnexpectedRequestLinePathError(fpath, giv string) error {
	return fmt.Errorf("Parser encountered a 'when' file '%s' with an unexpected path in the first line: '%s', expected absolute path (starting with '/')", fpath, giv)
}
