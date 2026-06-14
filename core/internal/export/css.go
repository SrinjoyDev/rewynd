package export

// css is the inline stylesheet for the exported trace — Catppuccin Mocha, self-contained.
const css = `
:root{
  --base:#1e1e2e;--mantle:#181825;--crust:#11111b;--surface:#313244;--overlay:#6c7086;
  --text:#cdd6f4;--sub:#a6adc8;--muted:#9399b2;
  --green:#a6e3a1;--red:#f38ba8;--yellow:#f9e2af;--blue:#89b4fa;--mauve:#cba6f7;--sapphire:#74c7ec;
}
*{box-sizing:border-box}
body{margin:0;background:var(--crust);color:var(--text);
  font:15px/1.55 ui-sans-serif,system-ui,-apple-system,"Inter",sans-serif;}
main{max-width:920px;margin:0 auto;padding:32px 20px 64px}
a{color:var(--sapphire)}
header{display:flex;align-items:center;gap:12px;flex-wrap:wrap;
  background:var(--mantle);border:1px solid var(--surface);border-left-width:4px;
  border-radius:12px;padding:16px 20px;font-size:20px}
header.s-ok{border-left-color:var(--green)} header.s-err{border-left-color:var(--red)}
header.s-warn{border-left-color:var(--yellow)} header.s-na{border-left-color:var(--overlay)}
.method{font-weight:700;color:var(--text)}
.path{font-family:ui-monospace,monospace;color:var(--text);word-break:break-all}
.badge{background:var(--mauve);color:var(--crust);font-size:12px;font-weight:700;
  padding:2px 8px;border-radius:6px;letter-spacing:.5px}
.status{font-weight:700;margin-left:auto}
header.s-ok .status{color:var(--green)} header.s-err .status{color:var(--red)}
header.s-warn .status{color:var(--yellow)}
.dur{color:var(--sub);font-size:15px}
.meta{display:flex;gap:18px;flex-wrap:wrap;color:var(--muted);font-size:13px;margin:12px 4px 4px}
.m-k{color:var(--overlay)}
h2{font-size:13px;text-transform:uppercase;letter-spacing:1px;color:var(--mauve);
  margin:28px 4px 10px;font-weight:700}
.detection{background:var(--mantle);border:1px solid var(--surface);border-radius:10px;
  padding:12px 16px;margin:8px 0}
.d-title{color:var(--mauve);font-weight:600} .d-sugg{color:var(--sub);font-size:14px;margin-top:4px}
.exc{background:rgba(243,139,168,.07);border:1px solid rgba(243,139,168,.3);border-radius:10px;
  padding:12px 16px;margin:8px 0}
.e-msg{color:var(--red);font-weight:600}
.stack{margin:8px 0 0;color:var(--sub);font-size:13px;white-space:pre-wrap;overflow-x:auto}
.waterfall{background:var(--mantle);border:1px solid var(--surface);border-radius:10px;padding:8px 4px}
.wf-row{display:grid;grid-template-columns:minmax(140px,1.2fr) 3fr 64px;align-items:center;
  gap:12px;padding:5px 14px;font-size:13px}
.wf-row:nth-child(even){background:rgba(108,112,134,.06)}
.wf-stmt{font-family:ui-monospace,monospace;color:var(--text);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.svc{color:var(--blue);font-size:11px;margin-left:6px}
.wf-track{position:relative;height:14px;background:rgba(108,112,134,.12);border-radius:4px}
.wf-bar{position:absolute;top:0;height:14px;background:linear-gradient(90deg,var(--blue),var(--sapphire));border-radius:4px;min-width:3px}
.wf-dur{color:var(--sub);text-align:right;font-variant-numeric:tabular-nums}
table.rows,table.headers{width:100%;border-collapse:collapse;font-size:13px;
  background:var(--mantle);border:1px solid var(--surface);border-radius:10px;overflow:hidden}
table.rows td,table.headers td{padding:8px 14px;border-top:1px solid var(--surface)}
table.rows tr:first-child td,table.headers tr:first-child td{border-top:none}
.o-status{font-weight:700;width:48px} .o-status.s-ok{color:var(--green)} .o-status.s-err{color:var(--red)}
.o-status.s-warn{color:var(--yellow)}
.o-method{color:var(--sub);width:56px} .o-url{font-family:ui-monospace,monospace;word-break:break-all}
.o-dur{color:var(--sub);text-align:right;width:64px}
.h-key{color:var(--overlay);width:200px;font-family:ui-monospace,monospace}
.h-val{font-family:ui-monospace,monospace;word-break:break-all;color:var(--sub)}
.logs{background:var(--mantle);border:1px solid var(--surface);border-radius:10px;padding:8px 4px}
.log{padding:4px 14px;font-family:ui-monospace,monospace;font-size:13px}
.lvl{display:inline-block;width:46px;font-weight:700;font-size:11px;text-transform:uppercase}
.lvl-error,.lvl-fatal{color:var(--red)} .lvl-warn{color:var(--yellow)} .lvl-info{color:var(--green)}
.lvl-debug,.lvl-log{color:var(--overlay)}
pre.body{background:var(--mantle);border:1px solid var(--surface);border-radius:10px;
  padding:14px 16px;color:var(--sub);font-size:13px;white-space:pre-wrap;word-break:break-all;overflow-x:auto}
footer{margin-top:48px;text-align:center;color:var(--overlay);font-size:13px}
`
