"""Generate the reviewed API allowlist from a local DolphinScheduler OpenAPI file.
No server URLs, credentials, examples or deployment configuration are copied.
"""
import json, pathlib, sys, re

root = pathlib.Path(__file__).resolve().parent
spec = json.loads(pathlib.Path(sys.argv[1]).read_text(encoding='utf-8'))
P = '/projects/{projectCode}'
entries = [
 ('projects_list','get','/projects','read','projects'),
 ('project_get','get','/projects/{code}','read','projects'),
 ('project_create','post','/projects','write','projects'),
 ('project_update','put','/projects/{code}','write','projects'),
 ('workflows_list','get',P+'/process-definition','read','workflows'),
 ('workflow_get','get',P+'/process-definition/{code}','read','workflows'),
 ('workflow_tasks','get',P+'/process-definition/{code}/tasks','read','workflows'),
 ('workflow_versions','get',P+'/process-definition/{code}/versions','read','workflows'),
 ('workflow_create','post',P+'/process-definition','write','workflows'),
 ('workflow_update','put',P+'/process-definition/{code}','write','workflows'),
 ('workflow_release','post',P+'/process-definition/{code}/release','full','workflows'),
 ('workflow_start','post',P+'/executors/start-process-instance','full','instances'),
 ('instances_list','get',P+'/process-instances','read','instances'),
 ('instance_get','get',P+'/process-instances/{id}','read','instances'),
 ('instance_tasks','get',P+'/process-instances/{id}/tasks','read','instances'),
 ('instance_variables','get',P+'/process-instances/{id}/view-variables','read','instances'),
 ('instance_execute','post',P+'/executors/execute','full','instances'),
 ('tasks_list','get',P+'/task-instances','read','instances'),
 ('task_stop','post',P+'/task-instances/{id}/stop','full','instances'),
 ('task_definitions_list','get',P+'/task-definition','read','workflows'),
 ('task_definition_get','get',P+'/task-definition/{code}','read','workflows'),
 ('task_codes_generate','get',P+'/task-definition/gen-task-codes','write','workflows'),
 ('schedules_list','get',P+'/schedules','read','schedules'),
 ('schedule_create','post',P+'/schedules','write','schedules'),
 ('schedule_update','put',P+'/schedules/{id}','write','schedules'),
 ('schedule_online','post',P+'/schedules/{id}/online','full','schedules'),
 ('schedule_offline','post',P+'/schedules/{id}/offline','full','schedules'),
 ('schedule_delete','delete',P+'/schedules/{id}','full','schedules'),
 ('task_log','get','/log/{projectCode}/detail','read','instances'),
 ('worker_groups','get','/worker-groups/all','read','projects'),
 ('lineage_get','get',P+'/lineages/{workFlowCode}','read','workflows'),
]

def clean(schema):
    if '$ref' in schema:
        schema = spec['components']['schemas'][schema['$ref'].split('/')[-1]]
    out = {k:v for k,v in schema.items() if k in ('type','description','enum','default','minimum','maximum','minLength','maxLength')}
    if schema.get('format') == 'int64':
        out.update(type='string', pattern=r'^-?[0-9]+$')
        out['description'] = out.get('description','') + ' 64位整数使用十进制字符串，避免精度丢失。'
        out.pop('default',None)
    if 'items' in schema: out['items'] = clean(schema['items'])
    if 'properties' in schema:
        out['properties'] = {k:clean(v) for k,v in schema['properties'].items()}
        if 'required' in schema: out['required']=schema['required']
    # Swagger serializes these Java enums via toString(); Spring expects enum names.
    if any('Status{' in str(x) for x in out.get('enum',[])):
        out.pop('enum')
        out['description'] = '执行状态枚举名，例如 RUNNING_EXECUTION、SUCCESS、FAILURE。'
    return out

tools=[]
for name,method,path,access,group in entries:
    op=spec['paths'][path][method]
    params=[]; props={}; required=[]
    source_params=list(op.get('parameters',[]))
    for key in re.findall(r'\{([^}]+)\}',path):
        if not any(p['name']==key for p in source_params):
            source_params.append({'name':key,'in':'path','required':True,'schema':{'type':'integer','format':'int64'}})
    for p in source_params:
        if (name,p['name']) in [('workflow_update','otherParamsJson'),('task_definitions_list','searchWorkflowName'),('workflow_release','name'),('schedules_list','processDefinitionId')] and 'in' not in p:
            p=dict(p, **{'in':'query'})
        if name=='workflow_release' and p['name']=='name':
            p=dict(p, required=False)
        if name=='schedules_list' and p['name']=='processDefinitionId':
            p=dict(p, required=False)
        if p['in'] not in ('path','query'): raise ValueError(p)
        schema=clean(p.get('schema',{'type':'string'}))
        if p.get('description'): schema['description']=p['description']+' '+schema.get('description','')
        if p['name']=='pageNo': schema.update(default=1,minimum=1)
        if name=='workflow_start' and p['name']=='scheduleTime': schema.update(default='')
        if p['name']=='pageSize': schema.update(default=20,minimum=1,maximum=100)
        if name=='task_log' and p['name']=='skipLineNum': schema.update(default=0,minimum=0)
        if name=='task_log' and p['name']=='limit': schema.update(default=200,minimum=1,maximum=5000)
        if p['name']=='schedule': schema['description']='JSON字符串，含 startTime、endTime、crontab、timezoneId；日期格式 yyyy-MM-dd HH:mm:ss。'
        props[p['name']]=schema
        if p.get('required') and 'default' not in schema: required.append(p['name'])
        params.append({'name':p['name'],'in':p['in']})
    if 'requestBody' in op: raise ValueError('Review request body before enabling '+name)
    if access!='read':
        props['confirm']={'type':'boolean','const':True,'description':'确认执行此修改操作；只读连接仍会拒绝。'}
        required.append('confirm')
    tools.append({'name':name,'description':op.get('description',op.get('summary',name)),
        'method':method.upper(),'path':path,'access':access,'group':group,'params':params,
        'inputSchema':{'type':'object','properties':props,'required':required,'additionalProperties':False},
        'annotations':{'readOnlyHint':access=='read','destructiveHint':access=='full','openWorldHint':True}})
(root/'backend'/'tools.json').write_text(json.dumps(tools,ensure_ascii=False,indent=2),encoding='utf-8')
manifest=json.loads((root/'manifest.json').read_text(encoding='utf-8'))
manifest['permissions']=[]
manifest['description']='DolphinScheduler 项目、工作流、实例、调度与日志；连接由用户自行配置。'
manifest['entrypoints']['backend']['transport']='stdio-jsonl'
manifest['entrypoints']['backend']['protocol_versions']=[1]
manifest['entrypoints']['ui']['entry']='ui/workbench.html'
provider=manifest['contributions'][0]
provider.update(label='DolphinScheduler 连接',database_type='dolphinscheduler',description='填写你自己的 DolphinScheduler API 地址和 Token。')
provider['fields']=[
 {'key':'display_name','label':'连接名称','type':'text','binding':'name','required':True},
 {'key':'host','label':'API 主机','type':'text','binding':'host','required':True,'placeholder':'域名或 IP，不含 http:// 和路径'},
 {'key':'port','label':'API 端口','type':'number','binding':'port','default':12345,'required':True},
 {'key':'scheme','label':'协议','type':'select','binding':'config','default':'http','options':[{'label':'HTTP','value':'http'},{'label':'HTTPS','value':'https'}]},
 {'key':'api_path','label':'API 根路径','type':'text','binding':'config','default':'/dolphinscheduler','description':'反向代理可填自定义路径；根路径服务填 /。'},
 {'key':'api_token','label':'API Token','type':'password','binding':'secret','required':True,'description':'在 DolphinScheduler 中创建的访问令牌，保存在 DBX 密钥存储中。'},
 {'key':'read_only','label':'只读连接','type':'boolean','binding':'config','default':True,'description':'开启时禁止启动、停止、发布和其他修改操作。'},
 {'key':'timeout_seconds','label':'请求超时（秒）','type':'number','binding':'config','default':30},
]
manifest['contributions'][1].pop('ai',None)
manifest['contributions'][1].update(label='DolphinScheduler 工作台',description='浏览工作流、运行实例、调度和任务日志。')
manifest['contributions'].append({'type':'mcp','id':manifest['id']+'.mcp','ai_tools':True,'external_tools':True,
 'toolsets':[{'id':x,'title':label} for x,label in [('projects','项目与环境'),('workflows','工作流定义'),('instances','运行实例与日志'),('schedules','定时调度')]],
 'tools':{t['name']:{'access':t['access'],'toolset':t['group']} for t in tools}})
manifest.pop('localizations',None)
(root/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2),encoding='utf-8')
print(f'Generated {len(tools)} reviewed tools; no deployment settings copied.')
