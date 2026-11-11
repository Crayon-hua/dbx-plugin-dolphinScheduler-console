import {readFileSync} from 'node:fs';
import {execFileSync} from 'node:child_process';
const manifest=JSON.parse(readFileSync('manifest.json','utf8'));
const args=['package','.'];
if(process.env.GITHUB_ACTIONS==='true'){
 const event=JSON.parse(readFileSync(process.env.GITHUB_EVENT_PATH,'utf8'));
 const tag=event.release?.tag_name||event.inputs?.release_tag||process.env.GITHUB_REF_NAME;
 if(tag!==`v${manifest.version}`)throw Error(`Tag ${tag} must match manifest version v${manifest.version}`);
 const name=`${manifest.id}-${manifest.version}-${process.env.DBX_PLUGIN_TARGET}.dbxp`;
 args.push('--artifact-url',`https://github.com/${process.env.GITHUB_REPOSITORY}/releases/download/${tag}/${name}`);
}
// The npm launcher is a JavaScript entrypoint, so no shell quoting is needed on Windows.
const launcher=process.env.DBX_PLUGIN_CLI_JS;
if(launcher)execFileSync(process.execPath,[launcher,...args],{stdio:'inherit'});
else execFileSync('dbx-plugin',args,{stdio:'inherit',shell:process.platform==='win32'});
