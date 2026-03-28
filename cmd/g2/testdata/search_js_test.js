const fs = require('fs');
const assert = require('assert');
const vm = require('vm');

const parserCode = fs.readFileSync('templates/site/search_parser.js', 'utf8');
const engineCode = fs.readFileSync('templates/site/search.js', 'utf8');

vm.runInThisContext(parserCode);
vm.runInThisContext(engineCode);

const engine = new SearchEngine();

// Mock documents
engine.documents = [
    {
        id: 1,
        full_name: "app-admin/ollama",
        version: "0.0.1",
        description: "Run LLMs locally",
        licenses: ["MIT"],
        arches: ["amd64", "arm64"],
        mask: "none",
        keywords: ["~amd64", "~arm64"],
        search_text: "app-admin/ollama run llms locally",
        uses: ["cuda", "rocm"],
        depends: ["dev-lang/go", "sys-libs/zlib"]
    },
    {
        id: 2,
        full_name: "dev-lang/go",
        version: "1.22.1",
        description: "The Go Programming Language",
        licenses: ["BSD"],
        arches: ["amd64", "arm64", "x86"],
        mask: "none",
        keywords: ["amd64", "arm64"],
        search_text: "dev-lang/go the go programming language",
        uses: [],
        depends: []
    },
    {
        id: 3,
        full_name: "sys-apps/systemd",
        version: "254",
        description: "System and service manager for Linux",
        licenses: ["LGPL-2.1"],
        arches: ["amd64"],
        mask: "hard",
        keywords: ["-amd64"],
        search_text: "sys-apps/systemd system and service manager for linux",
        uses: ["pam", "seccomp"],
        depends: ["sys-libs/pam"]
    }
];

// Test basic term search
let res = engine.search("ollama");
assert.strictEqual(res.length, 1);
assert.strictEqual(res[0].id, 1);

// Test field search
res = engine.search("license:BSD");
assert.strictEqual(res.length, 1);
assert.strictEqual(res[0].id, 2);

// Test AND implicit
res = engine.search("license:MIT app-admin");
assert.strictEqual(res.length, 1);
assert.strictEqual(res[0].id, 1);

// Test AND explicit
res = engine.search("license:MIT AND arch:amd64");
assert.strictEqual(res.length, 1);
assert.strictEqual(res[0].id, 1);

// Test explicit OR
res = engine.search("license:MIT OR license:BSD");
assert.strictEqual(res.length, 2);
assert.ok(res.find(d => d.id === 1));
assert.ok(res.find(d => d.id === 2));

// Test NOT
res = engine.search("arch:amd64 NOT mask:hard");
assert.strictEqual(res.length, 2);
assert.ok(res.find(d => d.id === 1));
assert.ok(res.find(d => d.id === 2));

// Test grouping
res = engine.search("(license:MIT OR license:BSD) AND arch:amd64");
assert.strictEqual(res.length, 2);

// Test version
res = engine.search("version:>1.0.0");
assert.strictEqual(res.length, 2); // go 1.22.1 and systemd 254
assert.ok(res.find(d => d.id === 2));
assert.ok(res.find(d => d.id === 3));

// Test keyword field
res = engine.search("keyword:~amd64");
assert.strictEqual(res.length, 1);
assert.strictEqual(res[0].id, 1);

// Test depends
res = engine.search("depends:dev-lang/go");
assert.strictEqual(res.length, 1);
assert.strictEqual(res[0].id, 1);

console.log("All JS tests passed!");

// Additional syntax tests
res = engine.search("!mask:hard");
assert.strictEqual(res.length, 2);

res = engine.search("-mask:hard");
assert.strictEqual(res.length, 2);

res = engine.search("'system manager'");
assert.strictEqual(res.length, 1);
assert.strictEqual(res[0].id, 3);
console.log("SUCCESS");
