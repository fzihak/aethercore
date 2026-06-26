with open('core/memory/engine.go', 'r') as f:
    content = f.read()
content = content.replace(
'''	var combined []llm.Message
	combined = append(combined, e.shortTermMem...)''',
'''	combined := make([]llm.Message, 0, len(e.shortTermMem)+3) // pre-alloc for short-term + up to 3 long-term
	combined = append(combined, e.shortTermMem...)'''
)
with open('core/memory/engine.go', 'w') as f:
    f.write(content)

with open('sdk/sdk_test.go', 'r') as f:
    content = f.read()

content = content.replace(
'''			if err == nil {
				successCount++
			} else if errors.Is(err, sdk.ErrModuleAlreadyLoaded) {
				errorCount++
			} else {
				t.Errorf("unexpected error: %v", err)
			}''',
'''			switch {
			case err == nil:
				successCount++
			case errors.Is(err, sdk.ErrModuleAlreadyLoaded):
				errorCount++
			default:
				t.Errorf("unexpected error: %v", err)
			}'''
)
with open('sdk/sdk_test.go', 'w') as f:
    f.write(content)
