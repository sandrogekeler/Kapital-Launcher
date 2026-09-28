export namespace models {
	
	export class AppSettings {
	    theme: string;
	    prismExecutable: string;
	    prismRoot: string;
	    profileName: string;
	    lastChapter: string;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.prismExecutable = source["prismExecutable"];
	        this.prismRoot = source["prismRoot"];
	        this.profileName = source["profileName"];
	        this.lastChapter = source["lastChapter"];
	    }
	}
	export class ChangelogEntry {
	    version: string;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new ChangelogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.summary = source["summary"];
	    }
	}
	export class WikiTeaser {
	    title: string;
	    line: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new WikiTeaser(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.line = source["line"];
	        this.path = source["path"];
	    }
	}
	export class Server {
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new Server(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	    }
	}
	export class Pack {
	    type: string;
	    loader: string;
	    minecraft: string;
	    mods?: number;
	    memoryGb?: number;
	    version?: string;
	    packwiz?: string;
	    mrpack?: string;
	
	    static createFrom(source: any = {}) {
	        return new Pack(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.loader = source["loader"];
	        this.minecraft = source["minecraft"];
	        this.mods = source["mods"];
	        this.memoryGb = source["memoryGb"];
	        this.version = source["version"];
	        this.packwiz = source["packwiz"];
	        this.mrpack = source["mrpack"];
	    }
	}
	export class Instance {
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new Instance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	    }
	}
	export class Chapter {
	    id: string;
	    number: string;
	    name: string;
	    era: string;
	    kind: string;
	    blurb: string;
	    state: string;
	    instance: Instance;
	    pack: Pack;
	    server?: Server;
	    wiki: WikiTeaser;
	    changelog: ChangelogEntry[];
	
	    static createFrom(source: any = {}) {
	        return new Chapter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.number = source["number"];
	        this.name = source["name"];
	        this.era = source["era"];
	        this.kind = source["kind"];
	        this.blurb = source["blurb"];
	        this.state = source["state"];
	        this.instance = this.convertValues(source["instance"], Instance);
	        this.pack = this.convertValues(source["pack"], Pack);
	        this.server = this.convertValues(source["server"], Server);
	        this.wiki = this.convertValues(source["wiki"], WikiTeaser);
	        this.changelog = this.convertValues(source["changelog"], ChangelogEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EngineInfo {
	    found: boolean;
	    executable: string;
	    version: string;
	    root: string;
	    source: string;
	
	    static createFrom(source: any = {}) {
	        return new EngineInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.found = source["found"];
	        this.executable = source["executable"];
	        this.version = source["version"];
	        this.root = source["root"];
	        this.source = source["source"];
	    }
	}
	
	export class Wiki {
	    baseUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Wiki(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseUrl = source["baseUrl"];
	    }
	}
	export class Manifest {
	    $schema?: string;
	    version: number;
	    wiki: Wiki;
	    chapters: Chapter[];
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.$schema = source["$schema"];
	        this.version = source["version"];
	        this.wiki = this.convertValues(source["wiki"], Wiki);
	        this.chapters = this.convertValues(source["chapters"], Chapter);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

