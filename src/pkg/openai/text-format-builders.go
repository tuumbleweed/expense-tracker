package openai

func TextAsPlain(verbosity TextVerbosity) TextOptions {
	return TextOptions{
		Format:    TextFormat{Type: TextFormatTypeText},
		Verbosity: verbosity,
	}
}

func TextAsJSONObject() TextOptions {
	return TextOptions{
		Format: TextFormat{Type: TextFormatTypeJSONObject},
	}
}

func TextAsJSONSchema(name string, schema map[string]any, strict bool) TextOptions {
	return TextOptions{
		Format: TextFormat{
			Type:   TextFormatTypeJSONSchema,
			Name:   name,   // <-- required here
			Schema: schema, // <-- raw schema object
			Strict: &strict,
		},
	}
}
