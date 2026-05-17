export namespace gui {
	
	export class DownloadRequest {
	    mode: string;
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
	
	    static createFrom(source: any = {}) {
	        return new DownloadRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.url = source["url"];
	        this.tsListPath = source["tsListPath"];
	        this.referer = source["referer"];
	        this.userAgent = source["userAgent"];
	        this.outputDir = source["outputDir"];
	        this.outputName = source["outputName"];
	        this.ffmpegPath = source["ffmpegPath"];
	        this.ffmpegAuto = source["ffmpegAuto"];
	        this.ffmpegDir = source["ffmpegDir"];
	        this.dryRun = source["dryRun"];
	        this.skipProbe = source["skipProbe"];
	    }
	}

}

