import os
import re

def fix_lint_issues():
    with open('sdk/sdk_test.go', 'r') as f:
        content = f.read()

    # fix ifElseChain
    content = re.sub(
        r'if err == nil \{\n\t\t\t\tsuccessCount\+\+\n\t\t\t\} else if errors\.Is\(err, sdk\.ErrModuleAlreadyLoaded\) \{\n\t\t\t\terrorCount\+\+\n\t\t\t\} else \{\n\t\t\t\tt\.Errorf\("unexpected error: %v", err\)\n\t\t\t\}',
        r'switch {\n\t\t\tcase err == nil:\n\t\t\t\tsuccessCount++\n\t\t\tcase errors.Is(err, sdk.ErrModuleAlreadyLoaded):\n\t\t\t\terrorCount++\n\t\t\tdefault:\n\t\t\t\tt.Errorf("unexpected error: %v", err)\n\t\t\t}',
        content, flags=re.MULTILINE
    )

    # fix for loop intrange
    content = re.sub(r'for i := 0; i < n; i\+\+ \{', r'for i := range n {', content)

    with open('sdk/sdk_test.go', 'w') as f:
        f.write(content)

    with open('core/memory/engine_test.go', 'r') as f:
        content = f.read()

    content = re.sub(r'for i := 0; i < 4; i\+\+ \{', r'for i := range 4 {', content)
    with open('core/memory/engine_test.go', 'w') as f:
        f.write(content)

    with open('core/llm/retry.go', 'r') as f:
        content = f.read()
    content = re.sub(r'for i := 0; i < p\.maxRetries; i\+\+ \{', r'for i := range p.maxRetries {', content)
    with open('core/llm/retry.go', 'w') as f:
        f.write(content)

if __name__ == '__main__':
    fix_lint_issues()
