import "./styles.css";
import type { DownloadMode, DownloadRequest, GUIEvent } from "./types";

const defaultRequest: DownloadRequest = {
  mode: "url",
  url: "",
  tsListPath: "",
  referer: "https://live.work.weixin.qq.com/",
  userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
  outputDir: ".",
  outputName: "",
  ffmpegPath: "",
  ffmpegAuto: true,
  ffmpegDir: "",
  dryRun: false,
  skipProbe: false,
};

let state: DownloadRequest = { ...defaultRequest };
let running = false;
let lastOutputPath = "";
let logs: string[] = ["系统就绪。优先粘贴 trans_video_url 的 MP4 或 video_url 的 m3u8。"];
let latestEvent: GUIEvent | null = null;

const root = document.querySelector<HTMLDivElement>("#app");
if (!root) {
  throw new Error("app root not found");
}
const appRoot = root;

function backend() {
  return window.go?.gui?.Backend;
}

function emitLog(line: string) {
  const now = new Date().toLocaleTimeString("zh-CN", { hour12: false });
  logs = [`[${now}] ${line}`, ...logs].slice(0, 120);
  render();
}

function updateField<K extends keyof DownloadRequest>(key: K, value: DownloadRequest[K]) {
  state = { ...state, [key]: value };
  render();
}

function requestPayload(): DownloadRequest {
  return {
    ...state,
    url: state.mode === "url" ? state.url.trim() : "",
    tsListPath: state.mode === "ts" ? state.tsListPath.trim() : "",
    outputDir: state.outputDir.trim() || ".",
  };
}

async function startDownload() {
  if (running) return;
  const payload = requestPayload();
  if (payload.mode === "url" && !payload.url) {
    emitLog("请先粘贴 MP4 或 m3u8 地址。");
    return;
  }
  if (payload.mode === "ts" && !payload.tsListPath) {
    emitLog("请先选择 TS 分片列表文件。");
    return;
  }
  try {
    running = true;
    latestEvent = null;
    lastOutputPath = "";
    render();
    await backend()?.StartDownload(payload);
    emitLog("任务已提交到下载引擎。");
  } catch (error) {
    running = false;
    emitLog(`启动失败：${String(error)}`);
  }
}

async function cancelDownload() {
  await backend()?.CancelDownload();
  running = false;
  emitLog("已请求取消当前任务。");
  render();
}

async function selectOutputDir() {
  const value = await backend()?.SelectOutputDir();
  if (value) updateField("outputDir", value);
}

async function selectTSListFile() {
  const value = await backend()?.SelectTSListFile();
  if (value) updateField("tsListPath", value);
}

async function openOutput() {
  if (!lastOutputPath) return;
  await backend()?.OpenOutputFile(lastOutputPath);
}

function eventSummary(event: GUIEvent) {
  const parts = [event.message];
  if (event.safeUrl) parts.push(event.safeUrl);
  if (event.outputPath) parts.push(`输出：${event.outputPath}`);
  if (event.contentType) parts.push(`类型：${event.contentType}`);
  if (event.speed) parts.push(`速度：${event.speed}`);
  return parts.join(" · ");
}

function installRuntimeListeners() {
  window.runtime?.EventsOn("download:event", (event: GUIEvent) => {
    latestEvent = event;
    if (event.outputPath) lastOutputPath = event.outputPath;
    emitLog(eventSummary(event));
  });
  window.runtime?.EventsOn("download:error", (event: GUIEvent) => {
    latestEvent = event;
    running = false;
    emitLog(`失败：${event.message}`);
  });
  window.runtime?.EventsOn("download:done", (event: GUIEvent) => {
    latestEvent = event;
    running = false;
    emitLog(event.message || "任务完成。");
  });
}

function progressPercent(event: GUIEvent | null) {
  if (!event) return 0;
  if (event.bytesTotal > 0) return Math.min(100, Math.round((event.bytesDone / event.bytesTotal) * 100));
  if (event.done) return 100;
  return running ? 18 : 0;
}

function formatBytes(value: number) {
  if (!value) return "—";
  const units = ["B", "KB", "MB", "GB"];
  let size = value;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit++;
  }
  return `${size.toFixed(unit === 0 ? 0 : 1)} ${units[unit]}`;
}

function modeButton(mode: DownloadMode, label: string, kicker: string) {
  const active = state.mode === mode;
  return `<button class="mode-card ${active ? "active" : ""}" data-mode="${mode}">
    <span>${label}</span>
    <small>${kicker}</small>
  </button>`;
}

function render() {
  const percent = progressPercent(latestEvent);
  appRoot.innerHTML = `
    <main class="shell">
      <section class="hero-panel">
        <div class="brand-mark">W</div>
        <div>
          <p class="eyebrow">WeCom Replay Console</p>
          <h1>媒体回放下载驾驶舱</h1>
          <p class="subtitle">复制 MP4 / m3u8 直链，或用 TS 分片列表兜底合并。所有敏感参数都会在日志里自动脱敏。</p>
        </div>
        <div class="signal ${running ? "live" : ""}"><span></span>${running ? "运行中" : "待命"}</div>
      </section>

      <section class="grid">
        <form class="input-panel">
          <div class="panel-title">
            <span>01</span>
            <h2>输入源</h2>
          </div>
          <div class="mode-grid">
            ${modeButton("url", "MP4 / m3u8", "首选：trans_video_url / video_url")}
            ${modeButton("ts", "TS 分片列表", "兜底：每行一个 .ts URL")}
          </div>
          ${state.mode === "url" ? `
            <label class="field wide">
              <span>媒体直链</span>
              <textarea id="url" rows="4" placeholder="https://1253731777.vod2.myqcloud.com/.../v.f62951.mp4">${state.url}</textarea>
            </label>
          ` : `
            <label class="field wide file-row">
              <span>TS 列表文件</span>
              <input id="tsListPath" value="${state.tsListPath}" placeholder="D:\\Videos\\ts.txt" />
              <button type="button" id="pickTS">选择文件</button>
            </label>
          `}

          <div class="two-col">
            <label class="field"><span>输出目录</span><input id="outputDir" value="${state.outputDir}" /></label>
            <button type="button" id="pickOut" class="inline-pick">选择目录</button>
          </div>
          <label class="field"><span>输出名称</span><input id="outputName" value="${state.outputName}" placeholder="课程回放" /></label>

          <details>
            <summary>高级请求与 ffmpeg 设置</summary>
            <label class="field"><span>Referer</span><input id="referer" value="${state.referer}" /></label>
            <label class="field"><span>User-Agent</span><input id="userAgent" value="${state.userAgent}" /></label>
            <label class="field"><span>ffmpeg 路径</span><input id="ffmpegPath" value="${state.ffmpegPath}" placeholder="可留空" /></label>
            <label class="field"><span>ffmpeg 缓存目录</span><input id="ffmpegDir" value="${state.ffmpegDir}" placeholder="可留空" /></label>
            <div class="switches">
              <label><input id="ffmpegAuto" type="checkbox" ${state.ffmpegAuto ? "checked" : ""} /> 自动安装 ffmpeg</label>
              <label><input id="skipProbe" type="checkbox" ${state.skipProbe ? "checked" : ""} /> 跳过探测</label>
              <label><input id="dryRun" type="checkbox" ${state.dryRun ? "checked" : ""} /> Dry-run</label>
            </div>
          </details>

          <div class="actions">
            <button type="button" id="start" class="primary" ${running ? "disabled" : ""}>${running ? "任务运行中" : "开始下载"}</button>
            <button type="button" id="cancel" class="secondary" ${running ? "" : "disabled"}>取消</button>
          </div>
        </form>

        <aside class="status-panel">
          <div class="panel-title">
            <span>02</span>
            <h2>实时状态</h2>
          </div>
          <div class="gauge" style="--progress:${percent}">
            <div class="gauge-core">
              <strong>${percent}%</strong>
              <span>${latestEvent?.message || "等待任务"}</span>
            </div>
          </div>
          <div class="metric-grid">
            <div><span>输出</span><strong>${lastOutputPath || "—"}</strong></div>
            <div><span>速度</span><strong>${latestEvent?.speed || "—"}</strong></div>
            <div><span>媒体时间</span><strong>${latestEvent?.outTime || "—"}</strong></div>
            <div><span>数据量</span><strong>${formatBytes(latestEvent?.totalSize || latestEvent?.bytesDone || 0)}</strong></div>
          </div>
          <button type="button" id="openOutput" class="open-output" ${lastOutputPath ? "" : "disabled"}>打开输出文件</button>
        </aside>
      </section>

      <section class="log-panel">
        <div class="panel-title">
          <span>03</span>
          <h2>事件日志</h2>
        </div>
        <div class="log-stream">${logs.map((line) => `<p>${line}</p>`).join("")}</div>
      </section>
    </main>
  `;
  bindEvents();
}

function bindEvents() {
  document.querySelectorAll<HTMLButtonElement>("[data-mode]").forEach((button) => {
    button.addEventListener("click", () => updateField("mode", button.dataset.mode as DownloadMode));
  });
  bindInput("url", "url");
  bindInput("tsListPath", "tsListPath");
  bindInput("outputDir", "outputDir");
  bindInput("outputName", "outputName");
  bindInput("referer", "referer");
  bindInput("userAgent", "userAgent");
  bindInput("ffmpegPath", "ffmpegPath");
  bindInput("ffmpegDir", "ffmpegDir");
  bindCheck("ffmpegAuto", "ffmpegAuto");
  bindCheck("skipProbe", "skipProbe");
  bindCheck("dryRun", "dryRun");
  document.querySelector("#start")?.addEventListener("click", startDownload);
  document.querySelector("#cancel")?.addEventListener("click", cancelDownload);
  document.querySelector("#pickOut")?.addEventListener("click", selectOutputDir);
  document.querySelector("#pickTS")?.addEventListener("click", selectTSListFile);
  document.querySelector("#openOutput")?.addEventListener("click", openOutput);
}

function bindInput(id: string, key: keyof DownloadRequest) {
  const element = document.querySelector<HTMLInputElement | HTMLTextAreaElement>(`#${id}`);
  element?.addEventListener("input", () => updateField(key, element.value as never));
}

function bindCheck(id: string, key: keyof DownloadRequest) {
  const element = document.querySelector<HTMLInputElement>(`#${id}`);
  element?.addEventListener("change", () => updateField(key, element.checked as never));
}

installRuntimeListeners();
render();
