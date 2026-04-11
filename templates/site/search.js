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
            // Do not preload all documents; they will be fetched on demand.
            this.loaded = true;
        } catch (e) {
            console.error("Failed to load search index manifest", e);
        }
    }

    async search(query) {
        if (!query.trim()) return [];
        const parser = new SearchParser(query);
        const ast = parser.parse();

        const allDocIDs = await this.evaluateAst(ast);
        if (!allDocIDs || allDocIDs.size === 0) return [];

        const results = await this.fetchDocsByIDs(allDocIDs);

        // Filter sequence matches if any (since index only handles terms)
        const filteredResults = results.filter(doc => this.matchDoc(doc, ast));

        const terms = this.getTerms(ast);
        filteredResults.forEach(doc => {
            doc._score = this.scoreDoc(doc, terms);
        });

        filteredResults.sort((a, b) => {
            if (a._score !== b._score) {
                return b._score - a._score;
            }
            if (a.full_name !== b.full_name) {
                return a.full_name.localeCompare(b.full_name);
            }
            return (b.version_sort_key || b.version).localeCompare(a.version_sort_key || a.version);
        });

        return filteredResults;
    }


    async fetchDocsByIDs(docIDs) {
        // Find which batch files we need to load
        // docs-0.json, docs-1.json...
        // We do not have a direct id -> batch mapping without fetching all batches if they are not uniform,
        // but earlier we updated batch generation. Wait, the manifest lists data_files.
        // Actually, since we didn't output a strict ID range in manifest, we can just fetch all data_files if we really have to,
        // but that defeats the purpose. Oh, wait! The user asked: "maintain a separate set of document detail files (e.g., docs/{id}.json or batched doc files) so the client can retrieve the full SearchDocument payload only for the IDs that match the query".
        // With docs-0.json containing variable IDs, we would need an ID map.
        // Let's implement fetchDocsByIDs to fetch all batches that could contain the IDs.
        // Alternatively, if we just fetch all batches for now (or a cache), we should probably index them.
        // Actually, let's keep track of loaded documents.
        let docsToReturn = [];
        let missingBatchFiles = new Set(this.manifest.data_files); // In a real app we'd map ID to file.
        // To be efficient, we can load batch files one by one until we found all our docs.

        // Let's just load them in parallel for now, but cache them
        if (!this._docCache) {
            this._docCache = new Map();
            this._loadedBatches = new Set();
        }

        // Return docs from cache
        let missingIds = new Set(docIDs);
        for (const id of missingIds) {
            if (this._docCache.has(id)) {
                docsToReturn.push(this._docCache.get(id));
                missingIds.delete(id);
            }
        }

        if (missingIds.size > 0) {
            // Fetch individual doc files with controlled concurrency
            const idArray = Array.from(missingIds);
            const batchSize = 10;
            for (let i = 0; i < idArray.length; i += batchSize) {
                const batch = idArray.slice(i, i + batchSize);
                const promises = batch.map(id =>
                    fetch(`data/docs/${id}.json`)
                        .then(r => r.json())
                        .then(data => {
                            this._docCache.set(id, data);
                            docsToReturn.push(data);
                        })
                        .catch(e => console.error("Error fetching doc", id, e))
                );
                await Promise.all(promises);
            }
        }
        return docsToReturn;
    }

    getBucket(token) {
        let val = token;
        const colonIdx = token.indexOf(":");
        if (colonIdx !== -1) {
            val = token.substring(colonIdx + 1);
        }
        val = val.toLowerCase();
        let cleaned = "";
        for (let i = 0; i < val.length; i++) {
            const c = val[i];
            if ((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
                cleaned += c;
            }
        }
        if (cleaned.length === 0) return "_";
        if (cleaned.length === 1) return cleaned + "_";
        return cleaned.substring(0, 2);
    }

    async fetchIndexForTerm(term) {
        if (!term) return new Set();
        term = term.toLowerCase();
        const bucket = this.getBucket(term);
        if (!this._indexCache) this._indexCache = new Map();
        if (this._indexCache.has(bucket)) {
            const map = this._indexCache.get(bucket);
            return new Set(map[term] || []);
        }

        const p1 = bucket[0];
        const p2 = bucket.length > 1 ? bucket[1] : "_";
        try {
            const res = await fetch(`data/index/${p1}/${p2}/${bucket}.json`);
            if (!res.ok) return new Set();
            const map = await res.json();
            this._indexCache.set(bucket, map);
            return new Set(map[term] || []);
        } catch (e) {
            return new Set();
        }
    }

    async evaluateAst(ast) {
        if (!ast) return new Set();

        if (ast.type === 'OR') {
            const left = await this.evaluateAst(ast.left);
            const right = await this.evaluateAst(ast.right);
            // OR with a NOT doesn't make sense without a universe, so we ignore it if it's a NOT.
            if (left._isNot || right._isNot) {
                // Approximate: just return the positive side
                if (!left._isNot) return left;
                if (!right._isNot) return right;
                return new Set();
            }
            return new Set([...left, ...right]);
        }

        if (ast.type === 'AND') {
            const left = await this.evaluateAst(ast.left);
            const right = await this.evaluateAst(ast.right);

            if (left._isNot && right._isNot) return new Set(); // NOT AND NOT requires universe

            if (right._isNot) {
                return new Set([...left].filter(x => !right._childSet.has(x)));
            }
            if (left._isNot) {
                return new Set([...right].filter(x => !left._childSet.has(x)));
            }

            if (left.size === 0) return new Set(); // Short circuit
            return new Set([...left].filter(x => right.has(x)));
        }

        if (ast.type === 'NOT') {
            // Returning a negated set isn't possible directly with pure Set math unless we have a universe.
            // But we can mark it as a negation.
            // We can do this by wrapping the result:
            const childSet = await this.evaluateAst(ast.expr);
            const res = new Set();
            res._isNot = true;
            res._childSet = childSet;
            return res;
        }

        if (ast.type === 'GROUP') {
            return await this.evaluateAst(ast.expr);
        }

        if (ast.type === 'TERM') {
            return await this.fetchIndexForTerm(ast.value);
        }

        if (ast.type === 'FIELD') {
            return await this.fetchIndexForTerm(`${ast.field}:${ast.value}`);
        }

        if (ast.type === 'SEQUENCE') {
            // For sequences, fetch index for all words in sequence, AND them.
            const words = ast.value.toLowerCase().trim().split(/\s+/);
            if (words.length === 0) return new Set();
            let currentSet = await this.fetchIndexForTerm(words[0]);
            for (let i = 1; i < words.length; i++) {
                if (currentSet.size === 0) break;
                const wordSet = await this.fetchIndexForTerm(words[i]);
                currentSet = new Set([...currentSet].filter(x => wordSet.has(x)));
            }
            return currentSet;
        }

        return new Set();
    }

    getTerms(ast) {
        if (!ast) return [];
        switch(ast.type) {
            case 'OR':
            case 'AND':
                return this.getTerms(ast.left).concat(this.getTerms(ast.right));
            case 'NOT':
                return [];
            case 'GROUP':
                return this.getTerms(ast.expr);
            case 'TERM':
                return [ast.value.toLowerCase()];
            case 'FIELD':
                if (ast.field === 'category' || ast.field === 'overlay' || ast.field === 'arch' || ast.field === 'license' || ast.field === 'keyword') {
                    return [];
                }
                return [ast.value.toLowerCase()];
            case 'SEQUENCE':
                return [ast.value.toLowerCase()];
            default:
                return [];
        }
    }

    escapeRegExp(string) {
        return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    }

    scoreDoc(doc, terms) {
        let score = 0;
        const search_text = doc.search_text || "";
        const full_name = (doc.full_name || "").toLowerCase();
        for (const term of terms) {
            if (!term) continue;
            if (full_name === term || full_name.endsWith("/" + term)) {
                score += 100;
            } else if (full_name.includes(term)) {
                score += 50;
            } else if (new RegExp(`\\b${this.escapeRegExp(term)}\\b`, 'i').test(search_text)) {
                score += 10;
            } else if (search_text.includes(term)) {
                score += 1;
            }
        }
        return score;
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
            case 'bdepends': return (doc.bdepends || []).some(d => d.toLowerCase().includes(value));
            case 'pdepends': return (doc.pdepends || []).some(d => d.toLowerCase().includes(value));
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
