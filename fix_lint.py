with open('core/ipc_client.go', 'r') as f:
    content = f.read()

content = content.replace(
'''return "", fmt.Errorf("sandbox client not connected: call NewSandboxClient first")''',
'''return "", errors.New("sandbox client not connected: call NewSandboxClient first")'''
)

content = content.replace('''"fmt"''', '''"errors"\n\t"fmt"''')

with open('core/ipc_client.go', 'w') as f:
    f.write(content)
