with open('cmd/aether/auth.go', 'r') as f:
    content = f.read()

content = content.replace(
'''	srv, tokenChan := startAuthServer(state)

	// Open the browser
	state := generateState()''',
'''	// Open the browser
	state := generateState()

	srv, tokenChan := startAuthServer(state)'''
)

with open('cmd/aether/auth.go', 'w') as f:
    f.write(content)
