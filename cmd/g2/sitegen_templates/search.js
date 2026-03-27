class SearchEngine {
    constructor() {
        this.manifest = null;
        this.documents = [];
        this.loaded = false;
    }

    async init() {
        if (this.loaded) return;
        try {
            const res = await fetch('data/manifest.json');
            this.manifest = await res.json();

            for (const file of this.manifest.data_files) {
                const dataRes = await fetch(`data/${file}`);
                const data = await dataRes.json();
                this.documents = this.documents.concat(data);
            }
            this.loaded = true;
        } catch (e) {
            console.error("Failed to load search index", e);
        }
    }

    search(query) {
        if (!query.trim()) return [];
        const parser = new SearchParser(query);
        const ast = parser.parse();

        // Match against all documents
        const results = this.documents.filter(doc => this.matchDoc(doc, ast));

        // Rank results: simpler version, sort by length of full name to favor exact matches,
        // then by version string descending. (Or use VersionSortKey if we implement it client-side)
        results.sort((a, b) => {
            if (a.full_name !== b.full_name) {
                return a.full_name.localeCompare(b.full_name);
            }
            return b.version.localeCompare(a.version); // Basic fallback sort for versions
        });

        return results;
    }

    matchDoc(doc, ast) {
        if (!ast) return true;

        switch (ast.type) {
            case 'OR':
                return this.matchDoc(doc, ast.left) || this.matchDoc(doc, ast.right);
            case 'AND':
                return this.matchDoc(doc, ast.left) && this.matchDoc(doc, ast.right);
            case 'NOT':
                return !this.matchDoc(doc, ast.expr);
            case 'GROUP':
                return this.matchDoc(doc, ast.expr);
            case 'TERM':
                return this.matchTerm(doc, ast.value);
            case 'FIELD':
                return this.matchField(doc, ast.field, ast.value);
            case 'SEQUENCE':
                return this.matchSequence(doc, ast.value);
            default:
                return false;
        }
    }

    matchTerm(doc, term) {
        if (!term) return true;
        term = term.toLowerCase();
        return doc.search_text.includes(term);
    }

    matchField(doc, field, value) {
        value = value.toLowerCase();
        switch (field) {
            case 'overlay': return (doc.overlay || "").toLowerCase() === value;
            case 'category': return (doc.category || "").toLowerCase() === value;
            case 'url': return (doc.urls || []).some(u => u.toLowerCase().includes(value));
            case 'arch': return (doc.arches || []).some(a => a.toLowerCase() === value);
            case 'keyword': return (doc.keywords || []).some(k => k.toLowerCase() === value);
            case 'mask': return (doc.mask || "").toLowerCase() === value;
            case 'license': return (doc.licenses || []).some(l => l.toLowerCase() === value);
            case 'depends': return (doc.depends || []).some(d => d.toLowerCase().includes(value));
            case 'rdepends': return (doc.rdepends || []).some(d => d.toLowerCase().includes(value));
            case 'depended': return (doc.depended_by || []).some(d => d.toLowerCase().includes(value));
            case 'rdepended': return (doc.rdepended_by || []).some(d => d.toLowerCase().includes(value));
            case 'manifestfile': return (doc.manifest_files || []).some(m => m.toLowerCase() === value);
            case 'eapi': return (doc.eapi || "") === value;
            case 'slot': return (doc.slot || "") === value;
            case 'inherit': return (doc.inherits || []).some(i => i.toLowerCase() === value);
            case 'use': return (doc.uses || []).some(u => u.toLowerCase() === value);
            case 'version': return this.matchVersion(doc.version, value);
            default:
                // Fallback to text matching if field is unknown
                return this.matchTerm(doc, value);
        }
    }

    matchSequence(doc, seq) {
        seq = seq.toLowerCase();
        return doc.search_text.includes(seq);
    }

    matchVersion(docVersion, queryVersion) {
        // Support exact, >, <
        let op = "==";
        let v = queryVersion;
        if (queryVersion.startsWith(">=")) { op = ">="; v = queryVersion.substring(2); }
        else if (queryVersion.startsWith("<=")) { op = "<="; v = queryVersion.substring(2); }
        else if (queryVersion.startsWith(">")) { op = ">"; v = queryVersion.substring(1); }
        else if (queryVersion.startsWith("<")) { op = "<"; v = queryVersion.substring(1); }

        // Simple lexical comparison for now, assuming standard gentoo versions or sortable
        // Can be improved to use proper version segment comparison
        if (op === "==") return docVersion === v;
        if (op === ">") return docVersion > v;
        if (op === "<") return docVersion < v;
        if (op === ">=") return docVersion >= v;
        if (op === "<=") return docVersion <= v;

        return false;
    }
}
