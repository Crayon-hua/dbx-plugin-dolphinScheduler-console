"""Validate checked-in tool contracts and optionally a built .dbxp package."""
import hashlib,json,pathlib,sys,zipfile,re
root=pathlib.Path(__file__).resolve().parents[1]
m=json.loads((root/'manifest.json').read_text(encoding='utf-8'))
source=(root/'backend/main.go').read_text(encoding='utf-8')
assert re.search(r'const pluginID = "([^"]+)"',source).group(1)==m['id']
assert re.search(r'const version = "([^"]+)"',source).group(1)==m['version']
tools=json.loads((root/'backend/tools.json').read_text(encoding='utf-8'))
import jsonschema
jsonschema.Draft202012Validator(json.loads((root/'schemas/manifest.schema.json').read_text(encoding='utf-8'))).validate(m)
decl=next(x for x in m['contributions'] if x['type']=='mcp')['tools']
assert 0<len(tools)<=64 and len({t['name'] for t in tools})==len(tools)
assert set(decl)=={t['name'] for t in tools}
for t in tools:
    assert decl[t['name']]['access']==t['access']
    assert t['annotations']['readOnlyHint']==(t['access']=='read')
    jsonschema.Draft202012Validator.check_schema(t['inputSchema'])
    if t['access']!='read':
        assert 'confirm' in t['inputSchema']['required']
        assert t['inputSchema']['properties']['confirm']['const'] is True
    for p in t['params']:
        assert p['name'] in t['inputSchema']['properties']
provider=next(x for x in m['contributions'] if x['type']=='connection-provider')
fields={x['key']:x for x in provider['fields']}
assert fields['api_token']['binding']=='secret' and 'default' not in fields['api_token']
assert 'default' not in fields['host'] and fields['read_only']['default'] is True
for name in sys.argv[1:]:
    path=pathlib.Path(name)
    with zipfile.ZipFile(path) as z:
        assert 'signature.json' not in z.namelist()
        package=json.loads(z.read('manifest.json'))
        assert package['id']==m['id'] and package['version']==m['version']
        assert package['entrypoints']['backend']['executable'] in z.namelist()
        assert package['entrypoints']['ui']['entry'] in z.namelist()
        assert all(not any(x in n for x in ('.env','.dbx-dev','connections.json','.git/')) for n in z.namelist())
    meta=json.loads(path.with_suffix('.artifact.json').read_text(encoding='utf-8'))
    data=path.read_bytes()
    assert meta['sha256']==hashlib.sha256(data).hexdigest() and meta['size']==len(data)
print('PASS: manifest schema, %d tool contracts, secret bindings and %d package(s)'%(len(tools),len(sys.argv)-1))
