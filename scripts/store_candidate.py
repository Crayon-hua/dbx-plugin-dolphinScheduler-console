"""Convert published release-candidates.json into a reviewable Store submission."""
import json,pathlib,sys,urllib.parse
root=pathlib.Path(__file__).resolve().parents[1]
m=json.loads((root/'manifest.json').read_text(encoding='utf-8'))
release=json.loads(pathlib.Path(sys.argv[1]).read_text(encoding='utf-8'))
for k in ('id','publisher','version'):
    if release['plugin'][k]!=m[k]:raise ValueError('Release identity mismatch: '+k)
targets=[]
for a in release['artifacts']:
    if urllib.parse.urlparse(a['url']).scheme!='https':raise ValueError('Candidate URL must be HTTPS')
    if len(a['sha256'])!=64 or any(c not in '0123456789abcdef' for c in a['sha256'].lower()):raise ValueError('Invalid SHA-256')
    if not isinstance(a['size'],int) or a['size']<=0:raise ValueError('Invalid package size')
    targets.append({k:a[k] for k in ('target','url','sha256','size')})
if not targets:raise ValueError('No target artifacts')
repo='https://github.com/Crayon-hua/dbx-plugin-dolphinScheduler-console'
candidate={'schemaVersion':1,'id':m['id'],'publisher':m['publisher'],'version':m['version'],'name':m['name'],
 'description':'DolphinScheduler workflow, instance, schedule and log console with 31 MCP tools.',
 'icon':repo+'/raw/refs/tags/v'+m['version']+'/assets/plugin.svg','tags':['dolphinscheduler','workflow','mcp'],
 'permissions':m.get('permissions',[]),'source':repo+'/tree/v'+m['version'],'homepage':repo,'license':'Apache-2.0',
 'localizations':{'zh-CN':{'name':'DolphinScheduler 工作台','description':m['description']}},'targets':targets}
out=root/'store-submission'
for path,data in [(out/'publishers'/('crayon-hua.json'),{'id':'crayon-hua','name':'Crayon-hua','status':'unverified'}),(out/'candidates'/(m['id']+'.json'),candidate)]:
    path.parent.mkdir(parents=True,exist_ok=True)
    if path.exists():raise FileExistsError('Refusing to overwrite '+str(path))
    path.write_text(json.dumps(data,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
print('Generated store-submission/; review before opening the DBX Store PR.')
