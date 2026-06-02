with open('cmd/aether/auth.go', 'r') as f:
    content = f.read()

content = content.replace(
    'tokenChan := make(chan string)',
    'tokenChan = make(chan string)'
)

content = content.replace(
    'srv := &http.Server{',
    'srv = &http.Server{'
)

with open('cmd/aether/auth.go', 'w') as f:
    f.write(content)
