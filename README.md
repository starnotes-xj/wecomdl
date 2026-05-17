# wecom-replay-downloader

企业微信直播回放下载辅助工具。

本工具只用于下载你已经有权限观看的企业微信直播回放，不绕过登录、访问控制或 DRM。

## 优先级

抓包后优先找以下地址：

1. `trans_video_url` 里的 MP4 地址
2. `video_url` 里的 m3u8 地址
3. 只有 MP4/m3u8 都不可用时，才使用 TS 分片列表兜底

例如 `new_get_liveinfo` 响应里常见字段：

```json
{
  "cloud_info": {
    "video_url": "https://1253731777.vod2.myqcloud.com/.../playlist_eof.m3u8",
    "files": [
      {
        "trans_video_url": "https://1253731777.vod2.myqcloud.com/.../v.f62951.mp4"
      }
    ]
  }
}
```

其中 `trans_video_url` 通常就是最方便的完整 MP4。

## 使用方式

### 下载 MP4

```bash
wecomdl --url "https://1253731777.vod2.myqcloud.com/.../v.f62951.mp4" --out D:\Videos --name "课程回放"
```

### 下载 m3u8

```bash
wecomdl --url "https://1253731777.vod2.myqcloud.com/.../playlist_eof.m3u8" --out D:\Videos --name "课程回放"
```

### TS 分片兜底合并

如果只能抓到多个 TS 分片，把同一组分片 URL 保存到文本文件，例如 `ts.txt`：

```text
https://1253731777.vod2.myqcloud.com/.../video.ts?start=0&end=100&type=mpegts&resolution=2080x1168
https://1253731777.vod2.myqcloud.com/.../video.ts?start=101&end=200&type=mpegts&resolution=2080x1168
https://1253731777.vod2.myqcloud.com/.../video.ts?start=201&end=300&type=mpegts&resolution=2080x1168
```

然后执行：

```bash
wecomdl --ts-list ts.txt --out D:\Videos --name "课程回放-ts"
```

工具会：

- 忽略空行和 `#` 开头的注释行
- 去重完全相同的 TS URL
- 按 URL 参数里的 `start`、`end` 排序
- 使用 ffmpeg concat 合并为 MP4

注意：TS 分片必须属于同一个视频、同一个清晰度、同一组文件，不要混入广告分片或其他清晰度分片。

## 常用参数

```text
--url          MP4 或 m3u8 地址
--ts-list      TS 分片 URL 列表文件；与 --url 二选一
--out          输出目录，默认当前目录
--name         输出文件名，不需要 .mp4；为空时自动生成
--referer      请求 Referer，默认 https://live.work.weixin.qq.com/
--user-agent   请求 User-Agent
--ffmpeg       指定 ffmpeg.exe 路径
--ffmpeg-auto  找不到 ffmpeg 时自动下载固定版本，默认 true
--ffmpeg-dir   ffmpeg 自动下载缓存目录
--dry-run      只打印 ffmpeg 参数，不实际下载
--skip-probe   跳过下载前媒体探测
--timeout      整体超时，例如 30m；0 表示不限制
```

`--url` 和 `--ts-list` 不能同时使用。

## 如何抓链接

推荐在 Burp 或浏览器网络面板里搜索：

```text
trans_video_url
video_url
.m3u8
.mp4
```

如果看到类似：

```text
https://1253731777.vod2.myqcloud.com/d9c271dcvodtranssh1253731777/.../v.f62951.mp4
```

直接复制给 `--url`。

如果看到：

```text
https://1253731777.vod2.myqcloud.com/.../playlist_eof.m3u8
```

也直接复制给 `--url`。

不建议为了收集 TS 分片而完整播放一遍视频。TS 合并只是兜底方案，优先使用 MP4 或 m3u8。

## ffmpeg

工具默认会从 `PATH` 查找 ffmpeg。找不到时会自动下载固定版本并校验 SHA256。

如果你已经安装 ffmpeg，可以显式指定：

```bash
wecomdl --url "https://example.com/video.mp4" --ffmpeg "D:\Tools\ffmpeg\bin\ffmpeg.exe"
```

## 注意事项

- 只下载你有权限观看的回放。
- 不要把广告 MP4 当成回放视频，广告路径里常见 `ads_svp_video`、`snssvpdownload` 等字样。
- 如果媒体请求返回 401/403，通常是链接或会话过期，需要重新打开回放后复制新的地址。
- 如果内容受 DRM 保护，本工具不会也不能绕过 DRM。
