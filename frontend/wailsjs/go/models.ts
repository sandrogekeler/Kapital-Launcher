export namespace models {
	
	export class AppSettings {
	    theme: string;
	    prismExecutable: string;
	    prismRoot: string;
	    profileName: string;
	    lastChapter: string;
	    packOverrides?: Record<string, string>;
	    serverChoices?: Record<string, string>;
	    joinServers?: string[];
	    disabledMods?: Record<string, Array<string>>;
	    loadingSplash?: boolean;
	    loadingSplashAvailable?: boolean;
	    loadingSplashOn?: boolean;
	    staticArt?: boolean;
	
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
	        this.packOverrides = source["packOverrides"];
	        this.serverChoices = source["serverChoices"];
	        this.joinServers = source["joinServers"];
	        this.disabledMods = source["disabledMods"];
	        this.loadingSplash = source["loadingSplash"];
	        this.loadingSplashAvailable = source["loadingSplashAvailable"];
	        this.loadingSplashOn = source["loadingSplashOn"];
	        this.staticArt = source["staticArt"];
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
	export class ServerAddress {
	    label: string;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerAddress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.address = source["address"];
	    }
	}
	export class Server {
	    addresses: ServerAddress[];
	    software: string;
	
	    static createFrom(source: any = {}) {
	        return new Server(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.addresses = this.convertValues(source["addresses"], ServerAddress);
	        this.software = source["software"];
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
	export class ModToggle {
	    name: string;
	    jarPrefix: string;
	
	    static createFrom(source: any = {}) {
	        return new ModToggle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.jarPrefix = source["jarPrefix"];
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
	    jvm?: string;
	    toggles?: ModToggle[];
	
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
	        this.jvm = source["jvm"];
	        this.toggles = this.convertValues(source["toggles"], ModToggle);
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
	export class ModToggleState {
	    name: string;
	    jarPrefix: string;
	    jars: string[];
	    disabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModToggleState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.jarPrefix = source["jarPrefix"];
	        this.jars = source["jars"];
	        this.disabled = source["disabled"];
	    }
	}
	export class ModFile {
	    name: string;
	    disabled: boolean;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new ModFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.disabled = source["disabled"];
	        this.size = source["size"];
	    }
	}
	export class ChapterMods {
	    chapterId: string;
	    mods: ModFile[];
	    toggles: ModToggleState[];
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChapterMods(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapterId = source["chapterId"];
	        this.mods = this.convertValues(source["mods"], ModFile);
	        this.toggles = this.convertValues(source["toggles"], ModToggleState);
	        this.running = source["running"];
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
	export class ChapterSettings {
	    maxMemoryMb: number;
	    jvm: string;
	
	    static createFrom(source: any = {}) {
	        return new ChapterSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxMemoryMb = source["maxMemoryMb"];
	        this.jvm = source["jvm"];
	    }
	}
	export class ChapterSettingsInfo {
	    chapterId: string;
	    settings: ChapterSettings;
	    machineMemoryMb: number;
	    prismDefaultMb: number;
	    packMemoryMb: number;
	    presets: string[];
	    running: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChapterSettingsInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapterId = source["chapterId"];
	        this.settings = this.convertValues(source["settings"], ChapterSettings);
	        this.machineMemoryMb = source["machineMemoryMb"];
	        this.prismDefaultMb = source["prismDefaultMb"];
	        this.packMemoryMb = source["packMemoryMb"];
	        this.presets = source["presets"];
	        this.running = source["running"];
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
	export class GameState {
	    chapterId: string;
	    phase: string;
	    since: string;
	    startedAt: string;
	    exitCode?: number;
	    reason?: string;
	    splash?: boolean;
	    estimate?: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new GameState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapterId = source["chapterId"];
	        this.phase = source["phase"];
	        this.since = source["since"];
	        this.startedAt = source["startedAt"];
	        this.exitCode = source["exitCode"];
	        this.reason = source["reason"];
	        this.splash = source["splash"];
	        this.estimate = source["estimate"];
	    }
	}
	
	export class InstanceReport {
	    root: string;
	    dir: string;
	    present: Record<string, boolean>;
	    packUrl: Record<string, string>;
	    sizeBytes: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new InstanceReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.dir = source["dir"];
	        this.present = source["present"];
	        this.packUrl = source["packUrl"];
	        this.sizeBytes = source["sizeBytes"];
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
	
	
	
	
	export class PackChangelogEntry {
	    version: string;
	    date: string;
	    summary: string;
	    details: string;
	
	    static createFrom(source: any = {}) {
	        return new PackChangelogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.date = source["date"];
	        this.summary = source["summary"];
	        this.details = source["details"];
	    }
	}
	export class PackChangelog {
	    chapterId: string;
	    checked: boolean;
	    entries: PackChangelogEntry[];
	
	    static createFrom(source: any = {}) {
	        return new PackChangelog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapterId = source["chapterId"];
	        this.checked = source["checked"];
	        this.entries = this.convertValues(source["entries"], PackChangelogEntry);
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
	
	export class PackState {
	    chapterId: string;
	    installed: boolean;
	    checked: boolean;
	    upToDate: boolean;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new PackState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapterId = source["chapterId"];
	        this.installed = source["installed"];
	        this.checked = source["checked"];
	        this.upToDate = source["upToDate"];
	        this.version = source["version"];
	    }
	}
	export class PhaseTime {
	    phase: string;
	    ms: number;
	
	    static createFrom(source: any = {}) {
	        return new PhaseTime(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phase = source["phase"];
	        this.ms = source["ms"];
	    }
	}
	export class PreviewSituation {
	    id: string;
	    label: string;
	    scope: string;
	    card: boolean;
	    playsInstall: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PreviewSituation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.scope = source["scope"];
	        this.card = source["card"];
	        this.playsInstall = source["playsInstall"];
	    }
	}
	export class PreviewStart {
	    cardSkipped: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PreviewStart(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cardSkipped = source["cardSkipped"];
	    }
	}
	export class PrismRelease {
	    version: string;
	    asset: string;
	    url: string;
	    size: number;
	    digest: string;
	    page: string;
	    installed: string;
	    updateAvailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrismRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.asset = source["asset"];
	        this.url = source["url"];
	        this.size = source["size"];
	        this.digest = source["digest"];
	        this.page = source["page"];
	        this.installed = source["installed"];
	        this.updateAvailable = source["updateAvailable"];
	    }
	}
	export class RunLog {
	    kind: string;
	    name: string;
	    modifiedAt: string;
	    size: number;
	    crashed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RunLog(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.modifiedAt = source["modifiedAt"];
	        this.size = source["size"];
	        this.crashed = source["crashed"];
	    }
	}
	export class RunLogText {
	    kind: string;
	    name: string;
	    text: string;
	    offset: number;
	    size: number;
	    lines: number;
	    truncated: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RunLogText(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.text = source["text"];
	        this.offset = source["offset"];
	        this.size = source["size"];
	        this.lines = source["lines"];
	        this.truncated = source["truncated"];
	    }
	}
	export class RunReport {
	    game: GameState;
	    phases: PhaseTime[];
	    logTail: string;
	    logLines: number;
	    logTruncated: boolean;
	    crashReport: string;
	    consoleAvailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RunReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.game = this.convertValues(source["game"], GameState);
	        this.phases = this.convertValues(source["phases"], PhaseTime);
	        this.logTail = source["logTail"];
	        this.logLines = source["logLines"];
	        this.logTruncated = source["logTruncated"];
	        this.crashReport = source["crashReport"];
	        this.consoleAvailable = source["consoleAvailable"];
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
	
	
	export class ServerStatus {
	    chapterId: string;
	    checked: boolean;
	    online: boolean;
	    players: number;
	    max: number;
	    version: string;
	    motd: string;
	    latencyMs: number;
	    checkedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ServerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chapterId = source["chapterId"];
	        this.checked = source["checked"];
	        this.online = source["online"];
	        this.players = source["players"];
	        this.max = source["max"];
	        this.version = source["version"];
	        this.motd = source["motd"];
	        this.latencyMs = source["latencyMs"];
	        this.checkedAt = source["checkedAt"];
	    }
	}
	
	export class WikiPage {
	    title: string;
	    line: string;
	    url: string;
	    eras: string[];
	    id: string;
	    related: string[];
	
	    static createFrom(source: any = {}) {
	        return new WikiPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.line = source["line"];
	        this.url = source["url"];
	        this.eras = source["eras"];
	        this.id = source["id"];
	        this.related = source["related"];
	    }
	}
	export class WikiShot {
	    era: string;
	    src: string;
	    subject: string;
	
	    static createFrom(source: any = {}) {
	        return new WikiShot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.era = source["era"];
	        this.src = source["src"];
	        this.subject = source["subject"];
	    }
	}

}

