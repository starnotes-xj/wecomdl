declare global {
  interface Window {
    go?: {
      gui?: {
        Backend?: {
          StartDownload(request: DownloadRequest): Promise<void>;
          CancelDownload(): Promise<void>;
          SelectOutputDir(): Promise<string>;
          SelectTSListFile(): Promise<string>;
          OpenOutputFile(path: string): Promise<void>;
        };
      };
    };
    runtime?: {
      EventsOn(name: string, callback: (payload: GUIEvent) => void): void;
    };
  }
}

export type DownloadMode = "url" | "ts";

export interface DownloadRequest {
  mode: DownloadMode;
  url: string;
  tsListPath: string;
  referer: string;
  userAgent: string;
  outputDir: string;
  outputName: string;
  ffmpegPath: string;
  ffmpegAuto: boolean;
  ffmpegDir: string;
  dryRun: boolean;
  skipProbe: boolean;
}

export interface GUIEvent {
  kind: string;
  message: string;
  safeUrl: string;
  sourceKind: string;
  outputPath: string;
  ffmpegPath: string;
  bytesDone: number;
  bytesTotal: number;
  totalSize: number;
  outTime: string;
  speed: string;
  done: boolean;
  statusCode: number;
  contentType: string;
}

export {};
