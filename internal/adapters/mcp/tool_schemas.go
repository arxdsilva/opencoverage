package mcpadapter

func coverageIngestPayloadRequired() []string {
	return []string{"projectKey", "branch", "commitSha", "triggerType", "runTimestamp", "totalCoveragePercent", "packages"}
}

func coverageIngestPayloadProperties() map[string]any {
	return map[string]any{
		"projectKey":           map[string]any{"type": "string", "description": "Project identifier."},
		"projectName":          map[string]any{"type": "string", "description": "Project display name."},
		"projectGroup":         map[string]any{"type": "string", "description": "Optional project group."},
		"defaultBranch":        map[string]any{"type": "string", "description": "Default branch name."},
		"branch":               map[string]any{"type": "string", "description": "Branch for this run."},
		"commitSha":            map[string]any{"type": "string", "description": "Commit SHA."},
		"author":               map[string]any{"type": "string", "description": "Optional author."},
		"triggerType":          map[string]any{"type": "string", "description": "Run trigger type."},
		"runTimestamp":         map[string]any{"type": "string", "description": "RFC3339 run time."},
		"totalCoveragePercent": map[string]any{"type": "number", "description": "Overall coverage percent."},
		"thresholdPercent":     map[string]any{"type": "number", "description": "Optional project threshold override."},
		"packages": map[string]any{
			"type":     "array",
			"minItems": 1,
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"importPath":      map[string]any{"type": "string"},
					"coveragePercent": map[string]any{"type": "number"},
				},
				"required": []string{"importPath", "coveragePercent"},
			},
			"description": "Per-package coverage values.",
		},
	}
}

func integrationIngestPayloadRequired() []string {
	return []string{"projectKey", "branch", "commitSha", "triggerType", "runTimestamp", "ginkgoReport"}
}

func integrationIngestPayloadProperties() map[string]any {
	return map[string]any{
		"projectKey":    map[string]any{"type": "string", "description": "Project identifier."},
		"projectName":   map[string]any{"type": "string", "description": "Project display name."},
		"projectGroup":  map[string]any{"type": "string", "description": "Optional project group."},
		"defaultBranch": map[string]any{"type": "string", "description": "Default branch name."},
		"branch":        map[string]any{"type": "string", "description": "Branch for this run."},
		"commitSha":     map[string]any{"type": "string", "description": "Commit SHA."},
		"author":        map[string]any{"type": "string", "description": "Optional author."},
		"triggerType":   map[string]any{"type": "string", "description": "Run trigger type."},
		"runTimestamp":  map[string]any{"type": "string", "description": "RFC3339 run time."},
		"environment":   map[string]any{"type": "string", "description": "Optional environment (test, stage, prod, none)."},
		"ginkgoReport": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"ginkgoVersion":    map[string]any{"type": "string"},
				"suiteDescription": map[string]any{"type": "string"},
				"suitePath":        map[string]any{"type": "string"},
				"suiteSucceeded":   map[string]any{"type": "boolean"},
				"specialSuiteFailureReasons": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
				"specReports": map[string]any{
					"type":     "array",
					"minItems": 1,
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"leafNodeText":            map[string]any{"type": "string"},
							"containerHierarchyTexts": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
							"state":                   map[string]any{"type": "string"},
							"runTime":                 map[string]any{"type": "number"},
							"failure": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"message": map[string]any{"type": "string"},
									"location": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"fileName":   map[string]any{"type": "string"},
											"lineNumber": map[string]any{"type": "integer"},
										},
										"required": []string{"fileName", "lineNumber"},
									},
								},
								"required": []string{"message"},
							},
						},
						"required": []string{"leafNodeText", "state", "runTime"},
					},
				},
			},
			"required": []string{"suiteDescription", "suitePath", "specReports"},
		},
	}
}
