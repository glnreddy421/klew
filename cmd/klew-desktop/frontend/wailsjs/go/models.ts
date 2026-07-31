export namespace api {
	
	export class View {
	    summary: engine.IncidentSummary;
	    confidenceLabel: string;
	    hypothesis: string;
	    hypothesisReasons: string[];
	    hypothesisStatus: string;
	    hypothesisAlternatives: model.Hypothesis[];
	    confidenceTrend: string;
	    causalChain: string[];
	    nextChecks: string[];
	    correlation: string[];
	    signals: model.Signal[];
	    evidence: model.EvidenceEvent[];
	    logPatterns?: model.LogPatterns;
	    watching: number;
	    expectedWatches: number;
	    watchNote: string;
	    hypothesisChanges: number;
	    dropped: number;
	    updatedAt: string;
	    state: model.InvestigationState;
	    graph: render.GraphLayout;
	
	    static createFrom(source: any = {}) {
	        return new View(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = this.convertValues(source["summary"], engine.IncidentSummary);
	        this.confidenceLabel = source["confidenceLabel"];
	        this.hypothesis = source["hypothesis"];
	        this.hypothesisReasons = source["hypothesisReasons"];
	        this.hypothesisStatus = source["hypothesisStatus"];
	        this.hypothesisAlternatives = this.convertValues(source["hypothesisAlternatives"], model.Hypothesis);
	        this.confidenceTrend = source["confidenceTrend"];
	        this.causalChain = source["causalChain"];
	        this.nextChecks = source["nextChecks"];
	        this.correlation = source["correlation"];
	        this.signals = this.convertValues(source["signals"], model.Signal);
	        this.evidence = this.convertValues(source["evidence"], model.EvidenceEvent);
	        this.logPatterns = this.convertValues(source["logPatterns"], model.LogPatterns);
	        this.watching = source["watching"];
	        this.expectedWatches = source["expectedWatches"];
	        this.watchNote = source["watchNote"];
	        this.hypothesisChanges = source["hypothesisChanges"];
	        this.dropped = source["dropped"];
	        this.updatedAt = source["updatedAt"];
	        this.state = this.convertValues(source["state"], model.InvestigationState);
	        this.graph = this.convertValues(source["graph"], render.GraphLayout);
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

export namespace details {
	
	export class Field {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Field(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class KeyValue {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new KeyValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class Table {
	    columns: string[];
	    rows: string[][];
	
	    static createFrom(source: any = {}) {
	        return new Table(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	    }
	}
	export class Section {
	    id: string;
	    title: string;
	    group?: string;
	    fields?: Field[];
	    table?: Table;
	    keyValues?: KeyValue[];
	    notes?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Section(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.group = source["group"];
	        this.fields = this.convertValues(source["fields"], Field);
	        this.table = this.convertValues(source["table"], Table);
	        this.keyValues = this.convertValues(source["keyValues"], KeyValue);
	        this.notes = source["notes"];
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
	export class StatusBadge {
	    tone: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new StatusBadge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tone = source["tone"];
	        this.label = source["label"];
	    }
	}
	export class ObjectDetail {
	    ref: model.ObjectRef;
	    kind: string;
	    title: string;
	    category?: string;
	    status: StatusBadge;
	    summary?: Field[];
	    sections: Section[];
	
	    static createFrom(source: any = {}) {
	        return new ObjectDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = this.convertValues(source["ref"], model.ObjectRef);
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.category = source["category"];
	        this.status = this.convertValues(source["status"], StatusBadge);
	        this.summary = this.convertValues(source["summary"], Field);
	        this.sections = this.convertValues(source["sections"], Section);
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

export namespace engine {
	
	export class IncidentSummary {
	    context: string;
	    namespace: string;
	    query: string;
	    status: string;
	    leadingSignal: string;
	    likelyTrigger: string;
	    confidence: number;
	    readyPods: number;
	    unreadyPods: number;
	    affectedPods: number;
	    restarts: number;
	    endpointsReady: number;
	    endpointsTotal: number;
	    window: number;
	    live: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IncidentSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.namespace = source["namespace"];
	        this.query = source["query"];
	        this.status = source["status"];
	        this.leadingSignal = source["leadingSignal"];
	        this.likelyTrigger = source["likelyTrigger"];
	        this.confidence = source["confidence"];
	        this.readyPods = source["readyPods"];
	        this.unreadyPods = source["unreadyPods"];
	        this.affectedPods = source["affectedPods"];
	        this.restarts = source["restarts"];
	        this.endpointsReady = source["endpointsReady"];
	        this.endpointsTotal = source["endpointsTotal"];
	        this.window = source["window"];
	        this.live = source["live"];
	    }
	}

}

export namespace investigation {
	
	export class Relationship {
	    from: Ref;
	    to: Ref;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new Relationship(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = this.convertValues(source["from"], Ref);
	        this.to = this.convertValues(source["to"], Ref);
	        this.kind = source["kind"];
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
	export class RelatedCRD {
	    extension: string;
	    group: string;
	    kind: string;
	    refs: Ref[];
	
	    static createFrom(source: any = {}) {
	        return new RelatedCRD(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extension = source["extension"];
	        this.group = source["group"];
	        this.kind = source["kind"];
	        this.refs = this.convertValues(source["refs"], Ref);
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
	export class Ref {
	    kind: string;
	    name: string;
	    namespace?: string;
	    apiVersion?: string;
	
	    static createFrom(source: any = {}) {
	        return new Ref(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.apiVersion = source["apiVersion"];
	    }
	}
	export class InvestigationScope {
	    rootKind: string;
	    rootName: string;
	    namespace: string;
	    deployments?: Ref[];
	    statefulSets?: Ref[];
	    daemonSets?: Ref[];
	    replicaSets?: Ref[];
	    pods?: Ref[];
	    jobs?: Ref[];
	    cronJobs?: Ref[];
	    services?: Ref[];
	    ingresses?: Ref[];
	    endpoints?: Ref[];
	    endpointSlices?: Ref[];
	    nodes?: Ref[];
	    events?: Ref[];
	    configMaps?: Ref[];
	    secrets?: Ref[];
	    pvcs?: Ref[];
	    hpas?: Ref[];
	    serviceAccounts?: Ref[];
	    roles?: Ref[];
	    roleBindings?: Ref[];
	    pdbs?: Ref[];
	    networkPolicies?: Ref[];
	    relatedCRDs?: RelatedCRD[];
	    extensions?: string[];
	    relationships?: Relationship[];
	
	    static createFrom(source: any = {}) {
	        return new InvestigationScope(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rootKind = source["rootKind"];
	        this.rootName = source["rootName"];
	        this.namespace = source["namespace"];
	        this.deployments = this.convertValues(source["deployments"], Ref);
	        this.statefulSets = this.convertValues(source["statefulSets"], Ref);
	        this.daemonSets = this.convertValues(source["daemonSets"], Ref);
	        this.replicaSets = this.convertValues(source["replicaSets"], Ref);
	        this.pods = this.convertValues(source["pods"], Ref);
	        this.jobs = this.convertValues(source["jobs"], Ref);
	        this.cronJobs = this.convertValues(source["cronJobs"], Ref);
	        this.services = this.convertValues(source["services"], Ref);
	        this.ingresses = this.convertValues(source["ingresses"], Ref);
	        this.endpoints = this.convertValues(source["endpoints"], Ref);
	        this.endpointSlices = this.convertValues(source["endpointSlices"], Ref);
	        this.nodes = this.convertValues(source["nodes"], Ref);
	        this.events = this.convertValues(source["events"], Ref);
	        this.configMaps = this.convertValues(source["configMaps"], Ref);
	        this.secrets = this.convertValues(source["secrets"], Ref);
	        this.pvcs = this.convertValues(source["pvcs"], Ref);
	        this.hpas = this.convertValues(source["hpas"], Ref);
	        this.serviceAccounts = this.convertValues(source["serviceAccounts"], Ref);
	        this.roles = this.convertValues(source["roles"], Ref);
	        this.roleBindings = this.convertValues(source["roleBindings"], Ref);
	        this.pdbs = this.convertValues(source["pdbs"], Ref);
	        this.networkPolicies = this.convertValues(source["networkPolicies"], Ref);
	        this.relatedCRDs = this.convertValues(source["relatedCRDs"], RelatedCRD);
	        this.extensions = source["extensions"];
	        this.relationships = this.convertValues(source["relationships"], Relationship);
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

export namespace kube {
	
	export class ContextOption {
	    name: string;
	    cluster: string;
	    user: string;
	    namespace: string;
	    isCurrent: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContextOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.cluster = source["cluster"];
	        this.user = source["user"];
	        this.namespace = source["namespace"];
	        this.isCurrent = source["isCurrent"];
	    }
	}
	export class ClusterState {
	    kubeconfigPath: string;
	    currentContext: string;
	    selectedContext: string;
	    selectedNamespace: string;
	    cluster: string;
	    user: string;
	    contexts: ContextOption[];
	    namespaces: string[];
	    syncedAt: string;
	    syncError?: string;
	
	    static createFrom(source: any = {}) {
	        return new ClusterState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kubeconfigPath = source["kubeconfigPath"];
	        this.currentContext = source["currentContext"];
	        this.selectedContext = source["selectedContext"];
	        this.selectedNamespace = source["selectedNamespace"];
	        this.cluster = source["cluster"];
	        this.user = source["user"];
	        this.contexts = this.convertValues(source["contexts"], ContextOption);
	        this.namespaces = source["namespaces"];
	        this.syncedAt = source["syncedAt"];
	        this.syncError = source["syncError"];
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

export namespace main {
	
	export class DiscoverOptions {
	    query: string;
	    namespace: string;
	    kubeconfig: string;
	    context: string;
	
	    static createFrom(source: any = {}) {
	        return new DiscoverOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.namespace = source["namespace"];
	        this.kubeconfig = source["kubeconfig"];
	        this.context = source["context"];
	    }
	}
	export class OpenWindowOptions {
	    context: string;
	    namespace: string;
	    kubeconfig: string;
	
	    static createFrom(source: any = {}) {
	        return new OpenWindowOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.namespace = source["namespace"];
	        this.kubeconfig = source["kubeconfig"];
	    }
	}
	export class StartOptions {
	    query: string;
	    namespace: string;
	    allNamespaces: boolean;
	    kubeconfig: string;
	    context: string;
	    tail: number;
	    refreshSec: number;
	    windowSec: number;
	    maxLogRequests: number;
	    autoRefresh?: boolean;
	    useMetricsServer?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StartOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.namespace = source["namespace"];
	        this.allNamespaces = source["allNamespaces"];
	        this.kubeconfig = source["kubeconfig"];
	        this.context = source["context"];
	        this.tail = source["tail"];
	        this.refreshSec = source["refreshSec"];
	        this.windowSec = source["windowSec"];
	        this.maxLogRequests = source["maxLogRequests"];
	        this.autoRefresh = source["autoRefresh"];
	        this.useMetricsServer = source["useMetricsServer"];
	    }
	}

}

export namespace model {
	
	export class ActiveWatch {
	    name: string;
	    resource: string;
	    namespace?: string;
	    startedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ActiveWatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.resource = source["resource"];
	        this.namespace = source["namespace"];
	        this.startedAt = source["startedAt"];
	    }
	}
	export class ContainerStatus {
	    podName: string;
	    name: string;
	    image: string;
	    ready: boolean;
	    restartCount: number;
	    state: string;
	    reason?: string;
	    exitCode?: number;
	    startedAt?: string;
	    finishedAt?: string;
	    lastState?: string;
	    lastReason?: string;
	    lastExitCode?: number;
	    requestsCPU?: string;
	    requestsMem?: string;
	    limitsCPU?: string;
	    limitsMem?: string;
	    command?: string[];
	    args?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ContainerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.podName = source["podName"];
	        this.name = source["name"];
	        this.image = source["image"];
	        this.ready = source["ready"];
	        this.restartCount = source["restartCount"];
	        this.state = source["state"];
	        this.reason = source["reason"];
	        this.exitCode = source["exitCode"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.lastState = source["lastState"];
	        this.lastReason = source["lastReason"];
	        this.lastExitCode = source["lastExitCode"];
	        this.requestsCPU = source["requestsCPU"];
	        this.requestsMem = source["requestsMem"];
	        this.limitsCPU = source["limitsCPU"];
	        this.limitsMem = source["limitsMem"];
	        this.command = source["command"];
	        this.args = source["args"];
	    }
	}
	export class ObjectRef {
	    kind: string;
	    name: string;
	    namespace?: string;
	    uid?: string;
	
	    static createFrom(source: any = {}) {
	        return new ObjectRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.uid = source["uid"];
	    }
	}
	export class EventRecord {
	    timestamp: string;
	    type: string;
	    reason: string;
	    message: string;
	    count: number;
	    source?: string;
	    involvedObject: ObjectRef;
	
	    static createFrom(source: any = {}) {
	        return new EventRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.type = source["type"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	        this.count = source["count"];
	        this.source = source["source"];
	        this.involvedObject = this.convertValues(source["involvedObject"], ObjectRef);
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
	export class LogWord {
	    rank: number;
	    word: string;
	    count: number;
	    tf: number;
	    idf: number;
	    lift: number;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new LogWord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rank = source["rank"];
	        this.word = source["word"];
	        this.count = source["count"];
	        this.tf = source["tf"];
	        this.idf = source["idf"];
	        this.lift = source["lift"];
	        this.score = source["score"];
	    }
	}
	export class LogSample {
	    message: string;
	    pod: string;
	    container: string;
	    timestamp: string;
	    severity: string;
	
	    static createFrom(source: any = {}) {
	        return new LogSample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.pod = source["pod"];
	        this.container = source["container"];
	        this.timestamp = source["timestamp"];
	        this.severity = source["severity"];
	    }
	}
	export class LogTemplate {
	    id: string;
	    template: string;
	    count: number;
	    pct: number;
	    trend: string;
	    trendPct: number;
	    sparkline?: number[];
	    volumeHistory?: number[];
	    severity: string;
	    pods: string[];
	    samples: LogSample[];
	    keywords?: LogWord[];
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new LogTemplate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.template = source["template"];
	        this.count = source["count"];
	        this.pct = source["pct"];
	        this.trend = source["trend"];
	        this.trendPct = source["trendPct"];
	        this.sparkline = source["sparkline"];
	        this.volumeHistory = source["volumeHistory"];
	        this.severity = source["severity"];
	        this.pods = source["pods"];
	        this.samples = this.convertValues(source["samples"], LogSample);
	        this.keywords = this.convertValues(source["keywords"], LogWord);
	        this.score = source["score"];
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
	export class EvidenceCard {
	    evidenceId: string;
	    confidence: number;
	    rootEvent: LogTemplate;
	    triggeredLogs: LogTemplate[];
	
	    static createFrom(source: any = {}) {
	        return new EvidenceCard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.evidenceId = source["evidenceId"];
	        this.confidence = source["confidence"];
	        this.rootEvent = this.convertValues(source["rootEvent"], LogTemplate);
	        this.triggeredLogs = this.convertValues(source["triggeredLogs"], LogTemplate);
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
	export class EvidenceBoardPayload {
	    cards: EvidenceCard[];
	    windowMinutes: number;
	    threshold: number;
	    cardCount: number;
	    correlatedLogs: number;
	
	    static createFrom(source: any = {}) {
	        return new EvidenceBoardPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cards = this.convertValues(source["cards"], EvidenceCard);
	        this.windowMinutes = source["windowMinutes"];
	        this.threshold = source["threshold"];
	        this.cardCount = source["cardCount"];
	        this.correlatedLogs = source["correlatedLogs"];
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
	export class MetricsSummary {
	    available: boolean;
	    cpuRequestMillicores: number;
	    cpuLimitMillicores: number;
	    cpuUsageMillicores: number;
	    memRequestMi: number;
	    memLimitMi: number;
	    memUsageMi: number;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new MetricsSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.cpuRequestMillicores = source["cpuRequestMillicores"];
	        this.cpuLimitMillicores = source["cpuLimitMillicores"];
	        this.cpuUsageMillicores = source["cpuUsageMillicores"];
	        this.memRequestMi = source["memRequestMi"];
	        this.memLimitMi = source["memLimitMi"];
	        this.memUsageMi = source["memUsageMi"];
	        this.note = source["note"];
	    }
	}
	export class PermissionCheck {
	    resource: string;
	    verb: string;
	    namespace?: string;
	    allowed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PermissionCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.resource = source["resource"];
	        this.verb = source["verb"];
	        this.namespace = source["namespace"];
	        this.allowed = source["allowed"];
	    }
	}
	export class ResourceRef {
	    kind: string;
	    name: string;
	    namespace: string;
	    usedBy?: string;
	
	    static createFrom(source: any = {}) {
	        return new ResourceRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.usedBy = source["usedBy"];
	    }
	}
	export class HPASummary {
	    name: string;
	    namespace: string;
	    targetKind: string;
	    targetName: string;
	    minReplicas: number;
	    maxReplicas: number;
	    currentReplicas: number;
	    desiredReplicas: number;
	    atMax: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HPASummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.targetKind = source["targetKind"];
	        this.targetName = source["targetName"];
	        this.minReplicas = source["minReplicas"];
	        this.maxReplicas = source["maxReplicas"];
	        this.currentReplicas = source["currentReplicas"];
	        this.desiredReplicas = source["desiredReplicas"];
	        this.atMax = source["atMax"];
	    }
	}
	export class NodeSummary {
	    name: string;
	    ready: boolean;
	    memoryPressure: boolean;
	    diskPressure: boolean;
	    pidPressure: boolean;
	    unschedulable: boolean;
	    kubeletVersion?: string;
	    conditions?: string[];
	    allocatableCpuMillicores?: number;
	    allocatableMemMi?: number;
	    capacityCpuMillicores?: number;
	    capacityMemMi?: number;
	
	    static createFrom(source: any = {}) {
	        return new NodeSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ready = source["ready"];
	        this.memoryPressure = source["memoryPressure"];
	        this.diskPressure = source["diskPressure"];
	        this.pidPressure = source["pidPressure"];
	        this.unschedulable = source["unschedulable"];
	        this.kubeletVersion = source["kubeletVersion"];
	        this.conditions = source["conditions"];
	        this.allocatableCpuMillicores = source["allocatableCpuMillicores"];
	        this.allocatableMemMi = source["allocatableMemMi"];
	        this.capacityCpuMillicores = source["capacityCpuMillicores"];
	        this.capacityMemMi = source["capacityMemMi"];
	    }
	}
	export class LogRecord {
	    podName: string;
	    containerName: string;
	    previous: boolean;
	    lines: string[];
	    truncated: boolean;
	    collectedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new LogRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.podName = source["podName"];
	        this.containerName = source["containerName"];
	        this.previous = source["previous"];
	        this.lines = source["lines"];
	        this.truncated = source["truncated"];
	        this.collectedAt = source["collectedAt"];
	    }
	}
	export class IngressSummary {
	    name: string;
	    namespace: string;
	    hosts?: string[];
	    backends?: string[];
	
	    static createFrom(source: any = {}) {
	        return new IngressSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.hosts = source["hosts"];
	        this.backends = source["backends"];
	    }
	}
	export class ServiceSummary {
	    name: string;
	    namespace: string;
	    type: string;
	    clusterIP: string;
	    selector?: string;
	    readyEndpoints: number;
	    totalEndpoints: number;
	    ports?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ServiceSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.type = source["type"];
	        this.clusterIP = source["clusterIP"];
	        this.selector = source["selector"];
	        this.readyEndpoints = source["readyEndpoints"];
	        this.totalEndpoints = source["totalEndpoints"];
	        this.ports = source["ports"];
	    }
	}
	export class ReplicaSetSummary {
	    name: string;
	    namespace: string;
	    replicas: number;
	    ready: number;
	    deploymentOwner?: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ReplicaSetSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.replicas = source["replicas"];
	        this.ready = source["ready"];
	        this.deploymentOwner = source["deploymentOwner"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class PodSummary {
	    name: string;
	    namespace: string;
	    uid?: string;
	    node?: string;
	    phase: string;
	    ready: boolean;
	    restartCount: number;
	    containers: ContainerStatus[];
	    ownerRefs?: ObjectRef[];
	    labels?: Record<string, string>;
	    annotations?: Record<string, string>;
	    createdAt: string;
	    configMapRefs?: string[];
	    secretRefs?: string[];
	    pvcRefs?: string[];
	    relatedCRDKinds?: string[];
	
	    static createFrom(source: any = {}) {
	        return new PodSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.uid = source["uid"];
	        this.node = source["node"];
	        this.phase = source["phase"];
	        this.ready = source["ready"];
	        this.restartCount = source["restartCount"];
	        this.containers = this.convertValues(source["containers"], ContainerStatus);
	        this.ownerRefs = this.convertValues(source["ownerRefs"], ObjectRef);
	        this.labels = source["labels"];
	        this.annotations = source["annotations"];
	        this.createdAt = source["createdAt"];
	        this.configMapRefs = source["configMapRefs"];
	        this.secretRefs = source["secretRefs"];
	        this.pvcRefs = source["pvcRefs"];
	        this.relatedCRDKinds = source["relatedCRDKinds"];
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
	export class WorkloadSummary {
	    kind: string;
	    name: string;
	    namespace: string;
	    uid?: string;
	    replicas: number;
	    ready: number;
	    available: number;
	    updated: number;
	    generation: number;
	    observedGeneration: number;
	    selector?: string;
	    conditions?: string[];
	    labels?: Record<string, string>;
	    annotations?: Record<string, string>;
	    rootOwner?: ObjectRef;
	
	    static createFrom(source: any = {}) {
	        return new WorkloadSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.uid = source["uid"];
	        this.replicas = source["replicas"];
	        this.ready = source["ready"];
	        this.available = source["available"];
	        this.updated = source["updated"];
	        this.generation = source["generation"];
	        this.observedGeneration = source["observedGeneration"];
	        this.selector = source["selector"];
	        this.conditions = source["conditions"];
	        this.labels = source["labels"];
	        this.annotations = source["annotations"];
	        this.rootOwner = this.convertValues(source["rootOwner"], ObjectRef);
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
	export class MatchedObject {
	    ref: ObjectRef;
	    matchBy: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new MatchedObject(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = this.convertValues(source["ref"], ObjectRef);
	        this.matchBy = source["matchBy"];
	        this.score = source["score"];
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
	export class KubeContext {
	    context: string;
	    cluster: string;
	    user: string;
	    namespace: string;
	
	    static createFrom(source: any = {}) {
	        return new KubeContext(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.context = source["context"];
	        this.cluster = source["cluster"];
	        this.user = source["user"];
	        this.namespace = source["namespace"];
	    }
	}
	export class EvidenceBundle {
	    collectedAt: string;
	    kubeContext: KubeContext;
	    namespace: string;
	    query: string;
	    matchedObjects: MatchedObject[];
	    workloads: WorkloadSummary[];
	    pods: PodSummary[];
	    replicaSets: ReplicaSetSummary[];
	    services: ServiceSummary[];
	    ingresses: IngressSummary[];
	    events: EventRecord[];
	    logs: LogRecord[];
	    previousLogs: LogRecord[];
	    nodes: NodeSummary[];
	    nodePods?: PodSummary[];
	    hpas: HPASummary[];
	    configRefs: ResourceRef[];
	    secretRefs: ResourceRef[];
	    pvcRefs: ResourceRef[];
	    permissions: PermissionCheck[];
	    warnings: string[];
	    detectedCRDKinds?: string[];
	    metrics: MetricsSummary;
	
	    static createFrom(source: any = {}) {
	        return new EvidenceBundle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.collectedAt = source["collectedAt"];
	        this.kubeContext = this.convertValues(source["kubeContext"], KubeContext);
	        this.namespace = source["namespace"];
	        this.query = source["query"];
	        this.matchedObjects = this.convertValues(source["matchedObjects"], MatchedObject);
	        this.workloads = this.convertValues(source["workloads"], WorkloadSummary);
	        this.pods = this.convertValues(source["pods"], PodSummary);
	        this.replicaSets = this.convertValues(source["replicaSets"], ReplicaSetSummary);
	        this.services = this.convertValues(source["services"], ServiceSummary);
	        this.ingresses = this.convertValues(source["ingresses"], IngressSummary);
	        this.events = this.convertValues(source["events"], EventRecord);
	        this.logs = this.convertValues(source["logs"], LogRecord);
	        this.previousLogs = this.convertValues(source["previousLogs"], LogRecord);
	        this.nodes = this.convertValues(source["nodes"], NodeSummary);
	        this.nodePods = this.convertValues(source["nodePods"], PodSummary);
	        this.hpas = this.convertValues(source["hpas"], HPASummary);
	        this.configRefs = this.convertValues(source["configRefs"], ResourceRef);
	        this.secretRefs = this.convertValues(source["secretRefs"], ResourceRef);
	        this.pvcRefs = this.convertValues(source["pvcRefs"], ResourceRef);
	        this.permissions = this.convertValues(source["permissions"], PermissionCheck);
	        this.warnings = source["warnings"];
	        this.detectedCRDKinds = source["detectedCRDKinds"];
	        this.metrics = this.convertValues(source["metrics"], MetricsSummary);
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
	
	export class EvidenceEvent {
	    id: string;
	    timestamp: string;
	    sourceType: string;
	    sourceKind?: string;
	    sourceName?: string;
	    namespace?: string;
	    pod?: string;
	    container?: string;
	    node?: string;
	    severity: string;
	    reason?: string;
	    message: string;
	    raw?: string;
	    fingerprint?: string;
	    count?: number;
	    labels?: Record<string, string>;
	    annotations?: Record<string, string>;
	    confidence: number;
	    relatedObjectRefs?: ObjectRef[];
	
	    static createFrom(source: any = {}) {
	        return new EvidenceEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.timestamp = source["timestamp"];
	        this.sourceType = source["sourceType"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceName = source["sourceName"];
	        this.namespace = source["namespace"];
	        this.pod = source["pod"];
	        this.container = source["container"];
	        this.node = source["node"];
	        this.severity = source["severity"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	        this.raw = source["raw"];
	        this.fingerprint = source["fingerprint"];
	        this.count = source["count"];
	        this.labels = source["labels"];
	        this.annotations = source["annotations"];
	        this.confidence = source["confidence"];
	        this.relatedObjectRefs = this.convertValues(source["relatedObjectRefs"], ObjectRef);
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
	export class GraphEdge {
	    from: string;
	    to: string;
	    relation: string;
	    annotation?: string;
	
	    static createFrom(source: any = {}) {
	        return new GraphEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.relation = source["relation"];
	        this.annotation = source["annotation"];
	    }
	}
	export class GraphNode {
	    id: string;
	    kind: string;
	    name: string;
	    health: string;
	
	    static createFrom(source: any = {}) {
	        return new GraphNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.health = source["health"];
	    }
	}
	
	export class Hypothesis {
	    label: string;
	    category: string;
	    confidence: number;
	    leading: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Hypothesis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.category = source["category"];
	        this.confidence = source["confidence"];
	        this.leading = source["leading"];
	    }
	}
	export class HypothesisTransition {
	    from: string;
	    to: string;
	    confDelta: number;
	
	    static createFrom(source: any = {}) {
	        return new HypothesisTransition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	        this.confDelta = source["confDelta"];
	    }
	}
	
	export class LogPatternsWindow {
	    lineCount: number;
	    totalLogs: number;
	    patternCount: number;
	    podCount: number;
	    scope: string;
	    histMax: number;
	    wordModel?: string;
	    templateModel?: string;
	
	    static createFrom(source: any = {}) {
	        return new LogPatternsWindow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lineCount = source["lineCount"];
	        this.totalLogs = source["totalLogs"];
	        this.patternCount = source["patternCount"];
	        this.podCount = source["podCount"];
	        this.scope = source["scope"];
	        this.histMax = source["histMax"];
	        this.wordModel = source["wordModel"];
	        this.templateModel = source["templateModel"];
	    }
	}
	export class LogSeverityHist {
	    fatal: number;
	    error: number;
	    warn: number;
	    info: number;
	    debug: number;
	    trace: number;
	
	    static createFrom(source: any = {}) {
	        return new LogSeverityHist(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fatal = source["fatal"];
	        this.error = source["error"];
	        this.warn = source["warn"];
	        this.info = source["info"];
	        this.debug = source["debug"];
	        this.trace = source["trace"];
	    }
	}
	export class LogAttribute {
	    rank: number;
	    key: string;
	    count: number;
	    tf: number;
	    idf: number;
	    lift: number;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new LogAttribute(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rank = source["rank"];
	        this.key = source["key"];
	        this.count = source["count"];
	        this.tf = source["tf"];
	        this.idf = source["idf"];
	        this.lift = source["lift"];
	        this.score = source["score"];
	    }
	}
	export class LogPatterns {
	    templates: LogTemplate[];
	    words: LogWord[];
	    attributes: LogAttribute[];
	    severity: LogSeverityHist;
	    histogram: number[];
	    window: LogPatternsWindow;
	    eventTemplates?: LogTemplate[];
	    eventWords?: LogWord[];
	    eventReasons?: LogAttribute[];
	    eventSeverity?: LogSeverityHist;
	    eventHistogram?: number[];
	    eventWindow?: LogPatternsWindow;
	    evidenceBoard?: EvidenceBoardPayload;
	
	    static createFrom(source: any = {}) {
	        return new LogPatterns(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.templates = this.convertValues(source["templates"], LogTemplate);
	        this.words = this.convertValues(source["words"], LogWord);
	        this.attributes = this.convertValues(source["attributes"], LogAttribute);
	        this.severity = this.convertValues(source["severity"], LogSeverityHist);
	        this.histogram = source["histogram"];
	        this.window = this.convertValues(source["window"], LogPatternsWindow);
	        this.eventTemplates = this.convertValues(source["eventTemplates"], LogTemplate);
	        this.eventWords = this.convertValues(source["eventWords"], LogWord);
	        this.eventReasons = this.convertValues(source["eventReasons"], LogAttribute);
	        this.eventSeverity = this.convertValues(source["eventSeverity"], LogSeverityHist);
	        this.eventHistogram = source["eventHistogram"];
	        this.eventWindow = this.convertValues(source["eventWindow"], LogPatternsWindow);
	        this.evidenceBoard = this.convertValues(source["evidenceBoard"], EvidenceBoardPayload);
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
	export class StreamCounters {
	    eventsIngested: number;
	    logsIngested: number;
	    objectChanges: number;
	    metricSamples: number;
	    lastEventAt: string;
	
	    static createFrom(source: any = {}) {
	        return new StreamCounters(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.eventsIngested = source["eventsIngested"];
	        this.logsIngested = source["logsIngested"];
	        this.objectChanges = source["objectChanges"];
	        this.metricSamples = source["metricSamples"];
	        this.lastEventAt = source["lastEventAt"];
	    }
	}
	export class Signal {
	    id: string;
	    label: string;
	    severity: string;
	    strength: string;
	    source?: string;
	    count?: number;
	    score: number;
	    confidence?: number;
	    evidence: string;
	    objectRef?: ObjectRef;
	
	    static createFrom(source: any = {}) {
	        return new Signal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.severity = source["severity"];
	        this.strength = source["strength"];
	        this.source = source["source"];
	        this.count = source["count"];
	        this.score = source["score"];
	        this.confidence = source["confidence"];
	        this.evidence = source["evidence"];
	        this.objectRef = this.convertValues(source["objectRef"], ObjectRef);
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
	export class Verdict {
	    status: string;
	    leadingSignal?: string;
	    likelyTrigger: string;
	    confidence: number;
	    summary: string;
	    strongSignals: Signal[];
	    mediumSignals: Signal[];
	    weakSignals: Signal[];
	    affectedObjects: ObjectRef[];
	    affectedPods?: string[];
	    affectedServices?: string[];
	    recommendedNextChecks: string[];
	    missingDataWarnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Verdict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.leadingSignal = source["leadingSignal"];
	        this.likelyTrigger = source["likelyTrigger"];
	        this.confidence = source["confidence"];
	        this.summary = source["summary"];
	        this.strongSignals = this.convertValues(source["strongSignals"], Signal);
	        this.mediumSignals = this.convertValues(source["mediumSignals"], Signal);
	        this.weakSignals = this.convertValues(source["weakSignals"], Signal);
	        this.affectedObjects = this.convertValues(source["affectedObjects"], ObjectRef);
	        this.affectedPods = source["affectedPods"];
	        this.affectedServices = source["affectedServices"];
	        this.recommendedNextChecks = source["recommendedNextChecks"];
	        this.missingDataWarnings = source["missingDataWarnings"];
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
	export class TimelineEvent {
	    timestamp: string;
	    type: string;
	    severity: string;
	    sourceKind: string;
	    sourceName: string;
	    namespace: string;
	    message: string;
	    reason?: string;
	    involvedObject: ObjectRef;
	    confidence: number;
	    evidenceRefs: string[];
	
	    static createFrom(source: any = {}) {
	        return new TimelineEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.type = source["type"];
	        this.severity = source["severity"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceName = source["sourceName"];
	        this.namespace = source["namespace"];
	        this.message = source["message"];
	        this.reason = source["reason"];
	        this.involvedObject = this.convertValues(source["involvedObject"], ObjectRef);
	        this.confidence = source["confidence"];
	        this.evidenceRefs = source["evidenceRefs"];
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
	export class WorkloadGraph {
	    nodes: GraphNode[];
	    edges: GraphEdge[];
	    health: string;
	    healthByNode?: Record<string, string>;
	    propagationPath: string[];
	
	    static createFrom(source: any = {}) {
	        return new WorkloadGraph(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], GraphNode);
	        this.edges = this.convertValues(source["edges"], GraphEdge);
	        this.health = source["health"];
	        this.healthByNode = source["healthByNode"];
	        this.propagationPath = source["propagationPath"];
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
	export class RecentChange {
	    revisionFrom?: string;
	    revisionTo?: string;
	    deployedAt?: string;
	    image?: string;
	    helmRelease?: string;
	    helmRevision?: string;
	    gitSHA?: string;
	    syncState?: string;
	    rolloutStatus?: string;
	
	    static createFrom(source: any = {}) {
	        return new RecentChange(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.revisionFrom = source["revisionFrom"];
	        this.revisionTo = source["revisionTo"];
	        this.deployedAt = source["deployedAt"];
	        this.image = source["image"];
	        this.helmRelease = source["helmRelease"];
	        this.helmRevision = source["helmRevision"];
	        this.gitSHA = source["gitSHA"];
	        this.syncState = source["syncState"];
	        this.rolloutStatus = source["rolloutStatus"];
	    }
	}
	export class NamespaceScope {
	    allNamespaces: boolean;
	    namespaces?: string[];
	    primary: string;
	
	    static createFrom(source: any = {}) {
	        return new NamespaceScope(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allNamespaces = source["allNamespaces"];
	        this.namespaces = source["namespaces"];
	        this.primary = source["primary"];
	    }
	}
	export class InvestigationState {
	    collectedAt: string;
	    lastUpdatedAt: string;
	    mode: string;
	    kubeContext: KubeContext;
	    namespaceScope: NamespaceScope;
	    query: string;
	    window: number;
	    tailLines: number;
	    matchedObjects: MatchedObject[];
	    scope?: investigation.InvestigationScope;
	    recentChange?: RecentChange;
	    snapshot: EvidenceBundle;
	    workloadGraph: WorkloadGraph;
	    timeline: TimelineEvent[];
	    liveEvidence: EvidenceEvent[];
	    verdict: Verdict;
	    hypothesis: string;
	    hypothesisLabel: string;
	    hypothesisReasons: string[];
	    hypothesisStatus?: string;
	    hypothesisAlternatives?: Hypothesis[];
	    confidenceTrend?: string;
	    causalChain?: string[];
	    nextChecks?: string[];
	    fixActions?: string[];
	    lastTransition?: HypothesisTransition;
	    correlation: string[];
	    permissions: PermissionCheck[];
	    warnings: string[];
	    activeWatches: ActiveWatch[];
	    expectedWatches: number;
	    watchNote?: string;
	    counters: StreamCounters;
	    droppedEvidence: number;
	    hypothesisChanges: number;
	    paused: boolean;
	    logPatterns?: LogPatterns;
	
	    static createFrom(source: any = {}) {
	        return new InvestigationState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.collectedAt = source["collectedAt"];
	        this.lastUpdatedAt = source["lastUpdatedAt"];
	        this.mode = source["mode"];
	        this.kubeContext = this.convertValues(source["kubeContext"], KubeContext);
	        this.namespaceScope = this.convertValues(source["namespaceScope"], NamespaceScope);
	        this.query = source["query"];
	        this.window = source["window"];
	        this.tailLines = source["tailLines"];
	        this.matchedObjects = this.convertValues(source["matchedObjects"], MatchedObject);
	        this.scope = this.convertValues(source["scope"], investigation.InvestigationScope);
	        this.recentChange = this.convertValues(source["recentChange"], RecentChange);
	        this.snapshot = this.convertValues(source["snapshot"], EvidenceBundle);
	        this.workloadGraph = this.convertValues(source["workloadGraph"], WorkloadGraph);
	        this.timeline = this.convertValues(source["timeline"], TimelineEvent);
	        this.liveEvidence = this.convertValues(source["liveEvidence"], EvidenceEvent);
	        this.verdict = this.convertValues(source["verdict"], Verdict);
	        this.hypothesis = source["hypothesis"];
	        this.hypothesisLabel = source["hypothesisLabel"];
	        this.hypothesisReasons = source["hypothesisReasons"];
	        this.hypothesisStatus = source["hypothesisStatus"];
	        this.hypothesisAlternatives = this.convertValues(source["hypothesisAlternatives"], Hypothesis);
	        this.confidenceTrend = source["confidenceTrend"];
	        this.causalChain = source["causalChain"];
	        this.nextChecks = source["nextChecks"];
	        this.fixActions = source["fixActions"];
	        this.lastTransition = this.convertValues(source["lastTransition"], HypothesisTransition);
	        this.correlation = source["correlation"];
	        this.permissions = this.convertValues(source["permissions"], PermissionCheck);
	        this.warnings = source["warnings"];
	        this.activeWatches = this.convertValues(source["activeWatches"], ActiveWatch);
	        this.expectedWatches = source["expectedWatches"];
	        this.watchNote = source["watchNote"];
	        this.counters = this.convertValues(source["counters"], StreamCounters);
	        this.droppedEvidence = source["droppedEvidence"];
	        this.hypothesisChanges = source["hypothesisChanges"];
	        this.paused = source["paused"];
	        this.logPatterns = this.convertValues(source["logPatterns"], LogPatterns);
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

export namespace render {
	
	export class GraphStep {
	    id: string;
	    kind: string;
	    name: string;
	    label: string;
	    health: string;
	    relation?: string;
	
	    static createFrom(source: any = {}) {
	        return new GraphStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.label = source["label"];
	        this.health = source["health"];
	        this.relation = source["relation"];
	    }
	}
	export class LayoutNode {
	    id: string;
	    kind: string;
	    name: string;
	    health: string;
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new LayoutNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.health = source["health"];
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class GraphLayout {
	    nodes: LayoutNode[];
	    edges: model.GraphEdge[];
	    steps: GraphStep[];
	    width: number;
	    height: number;
	
	    static createFrom(source: any = {}) {
	        return new GraphLayout(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], LayoutNode);
	        this.edges = this.convertValues(source["edges"], model.GraphEdge);
	        this.steps = this.convertValues(source["steps"], GraphStep);
	        this.width = source["width"];
	        this.height = source["height"];
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

