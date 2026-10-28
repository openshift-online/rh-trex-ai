package generator

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	openapiEndpointStart        = "# NEW ENDPOINT START"
	openapiEndpointEnd          = "# NEW ENDPOINT END"
	openApiSchemaStart          = "# NEW SCHEMA START"
	openApiSchemaEnd            = "# NEW SCHEMA END"
	openApiEndpointMatchingLine = "  # AUTO-ADD NEW PATHS"
	openApiSchemaMatchingLine   = "    # AUTO-ADD NEW SCHEMAS"
)

// modifyOpenapi splices the paths and schemas defined in the generated per-kind
// OpenAPI file into the main openapi/openapi.yaml.
func modifyOpenapi(mainPath, kindPath string) error {
	endpointStrings := readBetweenLines(kindPath, openapiEndpointStart, openapiEndpointEnd)
	kindFileName := filepath.Base(kindPath)
	for _, line := range endpointStrings {
		endpointStr := strings.TrimSpace(line)
		endpointStr = strings.ReplaceAll(endpointStr, "/", "~1")
		endpointStr = strings.ReplaceAll(endpointStr, ":", "")
		refPath := fmt.Sprintf(`    $ref: '%s#/paths/%s'`, kindFileName, endpointStr)
		pathsLine := fmt.Sprintf("%s%s", line, refPath)
		if err := writeAfterLine(mainPath, openApiEndpointMatchingLine, pathsLine); err != nil {
			return err
		}
	}

	schemaStrings := readBetweenLines(kindPath, openApiSchemaStart, openApiSchemaEnd)
	for _, line := range schemaStrings {
		schemaStr := strings.TrimSpace(line)
		schemaStr = strings.ReplaceAll(schemaStr, ":", "")
		refPath := fmt.Sprintf(`      $ref: '%s#/components/schemas/%s'`, kindFileName, schemaStr)
		pathsLine := fmt.Sprintf("%s%s", line, refPath)
		if err := writeAfterLine(mainPath, openApiSchemaMatchingLine, pathsLine); err != nil {
			return err
		}
	}

	return nil
}

func readBetweenLines(path, startLine, endLine string) []string {
	readFile, err := os.Open(path)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer func() { _ = readFile.Close() }()

	fileScanner := bufio.NewScanner(readFile)
	fileScanner.Split(bufio.ScanLines)
	readFlag := false
	var totalMatches []string
	var matchedString strings.Builder
	for fileScanner.Scan() {
		trimmed := strings.TrimSpace(fileScanner.Text())
		if trimmed == startLine {
			readFlag = true
		} else if trimmed == endLine {
			readFlag = false
			totalMatches = append(totalMatches, matchedString.String())
			matchedString.Reset()
		} else if readFlag {
			matchedString.WriteString(fileScanner.Text() + "\n")
		}
	}
	return totalMatches
}

func writeAfterLine(path, matchingLine, lineToWrite string) error {
	input, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	output := bytes.ReplaceAll(input, []byte(matchingLine), []byte(lineToWrite+"\n"+matchingLine))
	return os.WriteFile(path, output, 0666)
}
