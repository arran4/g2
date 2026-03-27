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
            return (b.version_sort_key || b.version).localeCompare(a.version_sort_key || a.version);
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
            case 'version': return this.matchVersion(doc, value);
            default:
                // Fallback to text matching if field is unknown
                return this.matchTerm(doc, value);
        }
    }

    matchSequence(doc, seq) {
        seq = seq.toLowerCase();
        const words = seq.trim().split(/\s+/);
        if (words.length === 0) return true;

        let lastIndex = -1;
        for (const word of words) {
            const idx = doc.search_text.indexOf(word, lastIndex + 1);
            if (idx === -1) return false;
            lastIndex = idx;
        }
        return true;
    }

    matchVersion(doc, queryVersion) {
        // Support exact, >, <
        let op = "==";
        let v = queryVersion;
        if (queryVersion.startsWith(">=")) { op = ">="; v = queryVersion.substring(2); }
        else if (queryVersion.startsWith("<=")) { op = "<="; v = queryVersion.substring(2); }
        else if (queryVersion.startsWith(">")) { op = ">"; v = queryVersion.substring(1); }
        else if (queryVersion.startsWith("<")) { op = "<"; v = queryVersion.substring(1); }

        // We could write a small padding function here to match what Go does for `v`,
        // since `doc.version_sort_key` is already padded.
        const padVersion = (ver) => {
            if (!ver) return "";
            let pVer = ver.replace(/-r(\d+)$/, "+r$1");
            return pVer.replace(/\d+/g, (s) => s.padStart(10, "0"));
        };

        const docVersionPadded = doc.version_sort_key || padVersion(doc.version);
        const queryVersionPadded = padVersion(v);

        if (op === "==") return docVersionPadded === queryVersionPadded;
        if (op === ">") return docVersionPadded > queryVersionPadded;
        if (op === "<") return docVersionPadded < queryVersionPadded;
        if (op === ">=") return docVersionPadded >= queryVersionPadded;
        if (op === "<=") return docVersionPadded <= queryVersionPadded;

        return false;
    }
}
