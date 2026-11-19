import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const data = JSON.parse(fs.readFileSync(path.join(root, 'docs/features-mindmap.json'), 'utf8'))
const output = name => path.join(root, 'docs', 'features-mindmap' + name)
const esc = value => String(value).replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&apos;'}[char]))
const statusName = key => data.legend[key]
const tree = {id:'root',title:'渠智链 PartnerLedger',date:data.date,status:'ui',children:data.branches}
const leaves = node => node.children ? node.children.reduce((sum, child) => sum + leaves(child), 0) : 1
const walk = (node, fn, depth=0) => { fn(node,depth); node.children?.forEach(child => walk(child,fn,depth+1)) }
const all = []
walk(tree, node => all.push(node))
if (new Set(all.map(n=>n.id)).size !== all.length) throw Error('Duplicate feature IDs')
for (const node of all) {
  if (!data.legend[node.status]) throw Error('Unknown status ' + node.status)
  for (const source of node.sources || []) if (!fs.existsSync(path.join(root, source))) throw Error('Missing source: ' + source)
}

function makeMM(node, index=0) {
  const label = node.title + (node.id==='root' ? '' : ' ['+statusName(node.status)+']')
  const position = node.id!=='root' && data.branches.includes(node) ? ' POSITION="'+(index<6?'right':'left')+'"' : ''
  return '<node ID="'+node.id+'" TEXT="'+esc(label)+'"'+position+'>'+ (node.children?.map(makeMM).join('') || '')+'</node>'
}
fs.writeFileSync(output('.mm'), '<?xml version="1.0" encoding="UTF-8"?>\n<map version="1.0.1">'+makeMM(tree)+'</map>\n')

let outline = '# PartnerLedger 全功能思维导图\n\n'
outline += '核对日期：'+data.date+'。已提交基线：`'+data.baseline+'`；另列当前工作树中未提交的功能。共 '+data.branches.length+' 个主分支、'+leaves(tree)+' 个末级条目，条目包含功能、约束和边界，不代表独立模块数量。\n\n'
outline += '[打开交互导图](features-mindmap.html) · [完整矢量图](features-mindmap.svg) · [概览图](features-mindmap-overview.svg) · [FreeMind 格式](features-mindmap.mm)\n\n'
outline += data.notes.map(note=>'- '+note).join('\n')+'\n\n'
outline += '状态：'+Object.entries(data.legend).map(([key,label])=>label).join(' / ')+'。\n\n'
outline += '![功能导图概览](features-mindmap-overview.svg)\n\n<details>\n<summary>展开完整 Mermaid 思维导图</summary>\n\n```mermaid\nmindmap\n  root((渠智链 PartnerLedger))\n'
walk(tree,(node,depth)=>{ if(depth) outline+='  '.repeat(depth+1)+node.title.replace(/[()\[\]{}:]/g,' ')+' · '+statusName(node.status).split(' · ')[0]+'\n' })
outline += '```\n\n</details>\n\n## 可检索功能清单\n\n'
for (const branch of data.branches) {
  outline += '### '+branch.title+'\n\n'
  walk(branch,(node,depth)=>{ if(depth) outline+='  '.repeat(depth-1)+'- '+node.title+' **['+statusName(node.status)+']**\n' })
  outline += '\n代码依据：'+(branch.sources||[]).map(source=>'['+source+'](../'+source+')').join('、')+'。\n\n'
}
outline += '## 三方互通主线\n\n客户提交需求 → 代理商整理 → 厂商询价及补问 → 客户补资料 → 代理商回传。\n\n演示继续：厂商渠道报价 → 代理商对客报价 → 客户确认 → 代理商下单 → 厂商供货 → 代理商实施 → 客户验收／整改。\n\n副需求支线：拜访记录／厂商缺口 → 人工确认独立商机 → 团队伙伴匹配 → 联系前核对 → 双方同意引荐 → 合作结果与介绍费台账。\n\n维护：编辑 `docs/features-mindmap.json`，然后运行 `node scripts/build-feature-mindmap.mjs` 重建全部格式。\n'
fs.writeFileSync(output('.md'), outline)

// The same deterministic SVG renderer is embedded in the standalone viewer.
function renderMap(rootNode, legend, expanded, full=false) {
  const e = value => String(value).replace(/[&<>"']/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&apos;'}[c]))
  const count = n=>n.children ? n.children.reduce((s,c)=>s+count(c),0):1
  const visible = n=>n.children && (full || expanded.has(n.id)) ? n.children : []
  const labelLines = text=>{
    const result=[]; let line='', width=0
    for(const char of text){const add=char.charCodeAt(0)>255?1:.55;if(width+add>18&&line){result.push(line);line='';width=0}line+=char;width+=add}
    if(line)result.push(line)
    return result
  }
  const ownHeight = n=>Math.max(66, labelLines(n.title).length*21+37)
  const totalHeight = n=>Math.max(ownHeight(n), visible(n).reduce((s,c)=>s+totalHeight(c)+10,0)-10)
  const branches=visible(rootNode), right=branches.slice(0,Math.ceil(branches.length/2)), left=branches.slice(right.length)
  const sideHeight=nodes=>nodes.reduce((s,n)=>s+totalHeight(n)+26,0)-26
  const height=Math.max(500,sideHeight(left),sideHeight(right))+190
  const nodes=[], edges=[]
  function place(node, depth, dir, top, parent){
    const h=totalHeight(node), x=dir*(depth*345), y=top+h/2
    const point={node,x,y,height:ownHeight(node),width:305,dir,depth}; nodes.push(point)
    if(parent)edges.push({parent,child:point})
    const children=visible(node);let childTop=top+(h-(children.reduce((s,c)=>s+totalHeight(c)+10,0)-10))/2
    for(const child of children){place(child,depth+1,dir,childTop,point);childTop+=totalHeight(child)+10}
  }
  const center={node:rootNode,x:0,y:height/2,width:270,height:92,depth:0,dir:0};nodes.push(center)
  for(const [items,dir] of [[right,1],[left,-1]]){let top=(height-sideHeight(items))/2;for(const n of items){place(n,1,dir,top,center);top+=totalHeight(n)+26}}
  const minX=Math.min(...nodes.map(n=>n.x-n.width/2))-50,maxX=Math.max(...nodes.map(n=>n.x+n.width/2))+50,width=maxX-minX
  const color={live:'#005a9e',demo:'#805600',ui:'#526579',wip:'#7041a2',scope:'#526579'}
  const lines=edges.map(({parent:p,child:c})=>{const x1=p.x+c.dir*p.width/2,x2=c.x-c.dir*c.width/2,mid=(x1+x2)/2;return '<path d="M '+x1+' '+p.y+' C '+mid+' '+p.y+', '+mid+' '+c.y+', '+x2+' '+c.y+'" fill="none" stroke="#b6c8d9" stroke-width="1.5"/>'}).join('')
  const shapes=nodes.map(({node:n,x,y,width:w,height:h,depth})=>{
    const head=n.id==='root',has=!!n.children?.length,open=full||expanded.has(n.id),textLines=labelLines(n.title)
    const top=y-h/2
    const label=textLines.map((line,i)=>'<tspan x="'+(x-w/2+16)+'" y="'+(top+25+i*21)+'">'+e(line)+'</tspan>').join('')
    const subtitle=head ? '三端协作 · 渠道商机经营' : (has ? count(n)+' 个末级条目 · '+(open?'收起':'展开') : legend[n.status])
    return '<g data-id="'+e(n.id)+'"'+(has?' role="button" tabindex="0" aria-label="'+e(n.title)+'" aria-expanded="'+open+'"':'')+'><title>'+e(n.title+' · '+(legend[n.status]||''))+'</title><rect x="'+(x-w/2)+'" y="'+top+'" width="'+w+'" height="'+h+'" rx="'+(head?12:7)+'" fill="'+(head?'#102a43':depth===1?'#eaf4fc':'#ffffff')+'" stroke="'+(head?'#102a43':'#d7e2ec')+'"/><text font-size="'+(head?20:depth===1?18:15)+'" font-weight="'+(has?600:400)+'" fill="'+(head?'#f4fafd':'#17253d')+'">'+label+'</text><text x="'+(x-w/2+16)+'" y="'+(top+h-13)+'" font-size="12" fill="'+(head?'#bdd7eb':color[n.status])+'">'+e(subtitle)+'</text>'+(has&&!head?'<text x="'+(x+w/2-19)+'" y="'+(top+h-13)+'" font-size="16" fill="#005a9e">'+(open?'−':'+')+'</text>':'')+'</g>'
  }).join('')
  return {width,height,svg:'<svg xmlns="http://www.w3.org/2000/svg" width="'+width+'" height="'+height+'" viewBox="'+minX+' 0 '+width+' '+height+'" role="img" aria-label="PartnerLedger 功能思维导图"><style>text{font-family:Segoe UI,Microsoft YaHei,sans-serif}g[role=button]{cursor:pointer}g[role=button]:focus rect{stroke:#005a9e;stroke-width:3}</style><rect x="'+minX+'" y="0" width="'+width+'" height="'+height+'" fill="#f4f8fc"/><text x="'+(minX+28)+'" y="34" font-size="19" fill="#17253d">PartnerLedger 功能全景</text><text x="'+(minX+28)+'" y="58" font-size="13" fill="#526579">'+e(rootNode.date)+' · 正式 / 演示 / 前端 / 本机开发中 / 范围边界 · 状态见末级节点</text>'+lines+shapes+'</svg>'}
}
fs.writeFileSync(output('.svg'),renderMap(tree,data.legend,new Set(),true).svg)
fs.writeFileSync(output('-overview.svg'),renderMap(tree,data.legend,new Set(['root']),false).svg)

const html = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>PartnerLedger 全功能思维导图</title>
<style>
:root{font-family:'Segoe UI','Microsoft YaHei',sans-serif;color:#17253d;background:#f4f8fc;font-size:14px}*{box-sizing:border-box}body{margin:0}header{padding:20px 24px;background:white;border-bottom:1px solid #d7e2ec}h1{font-size:24px;margin:0 0 8px;font-weight:600}p{margin:6px 0;color:#526579;line-height:1.65}.toolbar{display:flex;flex-wrap:wrap;gap:8px;align-items:center;padding:12px 24px;background:white;border-bottom:1px solid #d7e2ec}button,input,select{font:inherit;padding:8px 12px;border:1px solid #7c8d9f;border-radius:6px;background:white;color:#17253d;min-height:38px}button{cursor:pointer}button:hover{background:#eaf4fc}input{width:210px;max-width:100%}:focus-visible{outline:2px solid #005a9e;outline-offset:2px}label{display:flex;gap:8px;align-items:center}.legend{display:flex;flex-wrap:wrap;gap:12px;padding:10px 24px;color:#526579}.status{font-size:12px}.workspace{display:grid;grid-template-columns:minmax(0,1fr) 290px;min-height:500px}.viewport{height:calc(100dvh - 254px);min-height:460px;overflow:auto;background:#f4f8fc}.drawing{transform-origin:top left}aside{padding:20px;border-left:1px solid #d7e2ec;background:white;overflow-wrap:anywhere}aside h2{font-size:18px;margin:0 0 10px}aside ul{padding-left:18px;line-height:1.8}aside a{color:#005a9e}#result{padding:0 24px 10px;color:#526579;font-size:12px}.mobile-outline{display:none}summary{padding:8px;cursor:pointer}#outline li{margin:6px 0}#outline ul{padding-left:20px}details>summary{color:#005a9e}#exports{display:flex;gap:8px;flex-wrap:wrap;margin-top:16px}#exports a{font-size:13px}footer{padding:10px 24px;font-size:12px;color:#526579}@media(max-width:760px){header,.toolbar,.legend{padding-inline:16px}.workspace{grid-template-columns:1fr}aside{border:0;border-top:1px solid #d7e2ec}.viewport{height:520px;min-height:0}h1{font-size:21px}.mobile-outline{display:block}button,input,select{min-height:44px}.toolbar label{flex-wrap:wrap}}@media print{.toolbar,.legend,aside,#result,.mobile-outline,footer{display:none}.workspace{display:block}.viewport{height:auto;overflow:visible}.drawing{transform:none!important}}
</style></head><body>
<header><h1>渠智链 · 全功能思维导图</h1><p>软件二级代理商的三端协作与商机经营平台</p><p class="status">${data.date} · 主分支 ${data.baseline} ＋ 单独标注的本机开发内容</p></header>
<div class="toolbar"><label>查找<input id="search" type="search" placeholder="如：报价、伙伴、验收"></label><label>范围<select id="filter"><option value="">所有状态</option>${Object.entries(data.legend).map(([key,value])=>'<option value="'+key+'">'+esc(value)+'</option>').join('')}</select></label><button id="expand">全部展开</button><button id="collapse">回到概览</button><button id="fit">适应窗口</button><button id="minus" aria-label="缩小导图">−</button><span id="zoomValue">100%</span><button id="plus" aria-label="放大导图">＋</button><button id="export">导出当前 SVG</button></div>
<div class="legend">${Object.values(data.legend).map(value=>'<span>'+esc(value)+'</span>').join('')}</div><div id="result" role="status" aria-live="polite"></div>
<div class="workspace"><div id="viewport" class="viewport"><div id="size"><div id="drawing" class="drawing"></div></div></div><aside><h2 id="selected">阅读说明</h2><div id="detail">点击分支逐层展开；可查找关键词或按实现状态查看。</div><p>正式已接入表示已连接真实 API，仍依赖后端与账号配置。演示中的报价、订单和验收尚未接入正式接口。</p><p>本机开发中为未提交工作，不计入已发布功能。</p><div id="exports"><a href="features-mindmap.svg" download>完整 SVG</a><a href="features-mindmap.mm" download>FreeMind 导入文件</a><a href="features-mindmap.md" download>Markdown 清单</a></div></aside></div>
<details class="mobile-outline"><summary>文字树与无障碍阅读</summary><div id="outline"></div></details><footer>数据依据：当前前端页面、Go 路由与业务动作、接口契约及交接文档。完整图含 ${leaves(tree)} 个末级条目（功能、约束及范围边界）。</footer>
<script>
const DATA=${JSON.stringify(data).replaceAll('<','\\u003c')};
const ROOT={id:'root',title:'渠智链 PartnerLedger',date:DATA.date,status:'ui',children:DATA.branches};
const renderMap=${renderMap.toString()};
const $=id=>document.getElementById(id), expanded=new Set(['root']), lookup=new Map();
function collect(n,parent){lookup.set(n.id,{...n,parent});n.children?.forEach(c=>collect(c,n.id))}collect(ROOT);
let current,scale=1,query='',filter='';
function textTree(n){const li=document.createElement('li');if(n.children){const d=document.createElement('details'),s=document.createElement('summary');s.textContent=n.title;d.append(s);const ul=document.createElement('ul');n.children.forEach(c=>ul.append(textTree(c)));d.append(ul);li.append(d)}else li.textContent=n.title+' ['+DATA.legend[n.status]+']';return li}const outline=document.createElement('ul');ROOT.children.forEach(n=>outline.append(textTree(n)));$('outline').append(outline);
function filtered(n){const match=n.title.toLocaleLowerCase().includes(query);const children=n.children?.map(filtered).filter(Boolean);if(!n.children)return (!query||match)&&(!filter||n.status===filter)?n:null;if(query&&match&&!filter){function reveal(x){expanded.add(x.id);x.children?.forEach(reveal)}reveal(n);return n}if(children?.length){if(query||filter)expanded.add(n.id);return {...n,children}}return n.id==='root'?{...n,children:[]}:null}
function resize(){const d=$('drawing');d.style.transform='scale('+scale+')';$('size').style.width=current.width*scale+'px';$('size').style.height=current.height*scale+'px';$('zoomValue').textContent=Math.round(scale*100)+'%'}
function draw(fit=false){const tree=filtered(ROOT);current=renderMap(tree,DATA.legend,expanded);$('drawing').innerHTML=current.svg;if(fit)scale=Math.min(1,Math.max(.35,($('viewport').clientWidth-16)/current.width));resize();$('result').textContent=(query||filter?'筛选结果 · ':'')+tree.children.length+' 个主分支；点击节点展开或收起。';$('drawing').querySelectorAll('g[data-id]').forEach(el=>{const choose=()=>{const n=lookup.get(el.dataset.id);if(n.children){expanded.has(n.id)?expanded.delete(n.id):expanded.add(n.id)}$('selected').textContent=n.title;$('detail').replaceChildren();const p=document.createElement('p');p.textContent=DATA.legend[n.status];$('detail').append(p);const ul=document.createElement('ul');(n.children||[]).forEach(c=>{const li=document.createElement('li');li.textContent=c.title;ul.append(li)});$('detail').append(ul);const sources=n.sources||lookup.get(n.parent)?.sources||[];sources.forEach(source=>{const p=document.createElement('p');p.textContent=source;$('detail').append(p)});draw(false)};el.addEventListener('click',choose);el.addEventListener('keydown',e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();choose();$('drawing').querySelector('[data-id="'+el.dataset.id+'"]')?.focus()}})})}
$('expand').onclick=()=>{lookup.forEach(n=>expanded.add(n.id));draw(false)};
$('collapse').onclick=()=>{expanded.clear();expanded.add('root');$('search').value='';$('filter').value='';query='';filter='';draw(true);$('viewport').scrollTo(0,0)};
$('search').oninput=e=>{query=e.target.value.trim().toLocaleLowerCase();draw(true)};
$('filter').onchange=e=>{filter=e.target.value;draw(true)};
$('fit').onclick=()=>{scale=Math.min(1,Math.max(.35,($('viewport').clientWidth-16)/current.width));resize();$('viewport').scrollTo(0,0)};
$('plus').onclick=()=>{scale=Math.min(1.8,scale+.1);resize()};$('minus').onclick=()=>{scale=Math.max(.25,scale-.1);resize()};
$('export').onclick=()=>{const url=URL.createObjectURL(new Blob([current.svg],{type:'image/svg+xml;charset=utf-8'}));const a=document.createElement('a');a.href=url;a.download='partnerledger-features.svg';a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)};
draw(true);
</script></body></html>`
fs.writeFileSync(output('.html'),html)
console.log('Built feature mindmap: '+data.branches.length+' branches, '+leaves(tree)+' leaf items, '+all.length+' total nodes.')
