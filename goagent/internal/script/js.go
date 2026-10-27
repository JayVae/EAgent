// Package script 依据选择器配置生成页面注入脚本。
// 生成结果可用配置里的 js_overrides 整体替换（完全开放的自定义通道）。
package script

import (
	"encoding/json"
	"strconv"
	"strings"

	"eagent-cli/internal/selectors"
)

// preload 生成公共的选择器工具函数。
func preload(p *selectors.Profile) string {
	selJSON, _ := json.Marshal(p.Selectors)
	return "var SEL=" + string(selJSON) + ";" + `
function pick(key){
  var list=SEL[key]||[]; if(typeof list==='string') list=[list];
  var out=[],seen=[];
  for(var i=0;i<list.length;i++){
    var els=document.querySelectorAll(list[i]);
    for(var j=0;j<els.length;j++){
      if(seen.indexOf(els[j])<0){seen.push(els[j]);out.push(els[j]);}
    }
  }
  return out;
}
function findSendButton(){
  var btns=pick('send_button'),btn=null;
  var w=window.innerWidth,h=window.innerHeight;
  var xmax=w*` + strconv.FormatFloat(p.Behavior.SendButtonXMaxRatio, 'f', -1, 64) + `;
  var ymin=h*` + strconv.FormatFloat(p.Behavior.SendButtonYMinRatio, 'f', -1, 64) + `;
  for(var i=0;i<btns.length;i++){
    var el=btns[i];
    if(el.offsetParent===null) continue;
    var r=el.getBoundingClientRect();
    if(r.x<xmax||r.y<ymin) continue;
    btn=el;
  }
  return btn;
}
`
}

func wrapOverride(p *selectors.Profile, key string) (string, bool) {
	if v := strings.TrimSpace(p.JsOverrides[key]); v != "" {
		return v, true
	}
	return "", false
}

// StateJS 返回读取页面状态的 JS：
// 结果 JSON: {a_count,u_count,last,has_input,send_state: send|stop|unknown}
func StateJS(p *selectors.Profile) string {
	if js, ok := wrapOverride(p, "get_state"); ok {
		return js
	}
	stopPrefix := ""
	if p.StopIconPrefix != "" {
		b, _ := json.Marshal(p.StopIconPrefix)
		stopPrefix = string(b)
	}
	return preload(p) + `
(function(){
  var as=pick('assistant_message');
  var last='';
  if(as.length) last=(as[as.length-1].textContent||'').trim();
  var us=pick('user_message');
  var inputs=pick('input');
  var sendState='unknown';
  var btn=findSendButton();
  if(btn){
    var svg=btn.querySelector('svg'),pp=svg&&svg.querySelector('path');
    var d=pp?(pp.getAttribute('d')||''):'';
    sendState=` + func() string {
		if stopPrefix == "" {
			return `'send'`
		}
		return `(d.indexOf(` + stopPrefix + `)===0)?'stop':'send'`
	}() + `;
  }
  return JSON.stringify({a_count:as.length,u_count:us.length,last:last,has_input:inputs.length>0,send_state:sendState});
})()`
}

// SendJS 生成"注入消息并发送"的 JS。msg 为空且存在按钮时走按钮点击
// （用于仅上传文件的轮次）。结果 JSON: {ok, err?, via?}
func SendJS(p *selectors.Profile, msg string) string {
	if js, ok := wrapOverride(p, "send"); ok {
		return js
	}
	msgJSON, _ := json.Marshal(msg)
	mode := p.Behavior.SendMode
	if mode == "" {
		mode = "auto"
	}
	return preload(p) + `
(function(){
  var msg=` + string(msgJSON) + `;
  var mode='` + mode + `';
  var ta=pick('input')[0]||null;
  if(msg!==''){
    if(!ta) return JSON.stringify({ok:false,err:'no textarea'});
    ta.focus();
    var s=Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype,'value').set;
    s.call(ta,msg);
    ta.dispatchEvent(new Event('input',{bubbles:true}));
    ta.dispatchEvent(new Event('change',{bubbles:true}));
  }
  if(msg===''||mode==='click'){
    var btn=findSendButton();
    if(!btn) return JSON.stringify({ok:false,err:'no send button'});
    btn.click();
    return JSON.stringify({ok:true,via:'click'});
  }
  function ev(t){
    var e=new KeyboardEvent(t,{key:'Enter',code:'Enter',keyCode:13,which:13,bubbles:true,cancelable:true});
    ta.dispatchEvent(e);
  }
  ev('keydown');ev('keypress');ev('keyup');
  return JSON.stringify({ok:true,via:'enter'});
})()`
}

// FileInputJS 用于定位页面文件选择框。
const FileInputJS = `document.querySelector('input[type="file"]')`

// AttachClickJS 点击配置了 attach_button 的附件按钮（可选能力）。
func AttachClickJS(p *selectors.Profile) string {
	return preload(p) + `
(function(){
  var b=pick('attach_button')[0]||null;
  if(!b) return JSON.stringify({ok:false,err:'no attach button'});
  b.click();
  return JSON.stringify({ok:true});
})()`
}
