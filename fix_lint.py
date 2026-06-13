with open('core/memory/zestdb_adapter.go', 'r') as f:
    content = f.read()

content = content.replace(
'''func (s *ZestDBStorage) Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryEntry, error) {''',
'''func (s *ZestDBStorage) Search(ctx context.Context, query string, opts SearchOptions) ([]MemoryEntry, error) {'''
)

import subprocess
import os

with open('core/memory/zestdb_adapter.go', 'w') as f:
    f.write(content)

os.system('go fmt core/memory/zestdb_adapter.go')
