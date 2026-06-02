with open('cmd/aether/auth.go', 'r') as f:
    content = f.read()

content = content.replace(
'''func startAuthServer(expectedState string) (*http.Server, chan string) {''',
'''//nolint:gocritic // named return values are unnecessary for these two standard variables
func startAuthServer(expectedState string) (*http.Server, chan string) {'''
)

with open('cmd/aether/auth.go', 'w') as f:
    f.write(content)
