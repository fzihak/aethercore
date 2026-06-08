with open('core/memory/zestdb_adapter.go', 'r') as f:
    content = f.read()

content = content.replace('// Search queries the in-memory persistence layer for entries matching the criteria.\n//nolint:gocognit // Mock search uses high cognitive complexity by design\nfunc (s *ZestDBStorage) Search', '// Search queries the in-memory persistence layer for entries matching the criteria.\n//nolint:gocognit // Mock search uses high cognitive complexity by design\n//\n//nolint:gocritic // Disable hugeParam check on opts in mock\nfunc (s *ZestDBStorage) Search')

with open('core/memory/zestdb_adapter.go', 'w') as f:
    f.write(content)
