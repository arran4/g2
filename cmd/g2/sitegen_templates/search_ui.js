document.addEventListener('DOMContentLoaded', async () => {
    const searchForm = document.getElementById('searchForm');
    const searchInput = document.getElementById('searchInput');
    const searchResults = document.getElementById('searchResults');
    const searchDocs = document.getElementById('searchDocs');
    const resultsCount = document.getElementById('searchResultsCount');

    const urlParams = new URLSearchParams(window.location.search);
    const initialQuery = urlParams.get('q');

    const engine = new SearchEngine();

    searchForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const query = searchInput.value;
        await performSearch(query);

        // Update URL
        const newUrl = new URL(window.location);
        newUrl.searchParams.set('q', query);
        window.history.pushState({}, '', newUrl);
    });

    async function performSearch(query) {
        if (!query || !query.trim()) {
            searchResults.innerHTML = '';
            resultsCount.textContent = '';
            searchDocs.style.display = 'block';
            return;
        }

        searchDocs.style.display = 'none';
        resultsCount.textContent = 'Searching...';
        searchResults.innerHTML = '';

        await engine.init();

        const results = engine.search(query);

        resultsCount.textContent = `Found ${results.length} results.`;

        if (results.length === 0) {
            searchResults.innerHTML = '<p>No results found for your query.</p>';
            return;
        }

        const fragment = document.createDocumentFragment();

        results.forEach(doc => {
            const resultDiv = document.createElement('div');
            resultDiv.style.border = '1px solid #eee';
            resultDiv.style.padding = '1em';
            resultDiv.style.marginBottom = '1em';
            resultDiv.style.borderRadius = '5px';
            resultDiv.style.backgroundColor = '#fcfcfc';

            const title = document.createElement('h3');
            title.style.marginTop = '0';
            const link = document.createElement('a');
            link.href = doc.page_url;
            link.textContent = `${doc.full_name}-${doc.version}`;
            title.appendChild(link);
            resultDiv.appendChild(title);

            const meta = document.createElement('p');
            meta.style.fontSize = '0.9em';
            meta.style.color = '#666';
            meta.style.margin = '0 0 0.5em 0';

            const badges = [];
            if (doc.overlay) badges.push(`<span style="background: #e0e0e0; padding: 2px 6px; border-radius: 4px; margin-right: 5px;">${doc.overlay}</span>`);
            if (doc.eapi) badges.push(`EAPI: ${doc.eapi}`);
            if (doc.slot) badges.push(`SLOT: ${doc.slot}`);
            if (doc.mask && doc.mask !== 'none') badges.push(`<span style="color: red;">MASK: ${doc.mask}</span>`);

            meta.innerHTML = badges.join(' | ');
            resultDiv.appendChild(meta);

            const desc = document.createElement('p');
            desc.textContent = doc.description;
            desc.style.marginBottom = '0.5em';
            resultDiv.appendChild(desc);

            if (doc.keywords && doc.keywords.length > 0) {
                const kw = document.createElement('p');
                kw.style.fontSize = '0.8em';
                kw.style.color = '#888';
                kw.innerHTML = `<strong>Keywords:</strong> ${doc.keywords.join(' ')}`;
                resultDiv.appendChild(kw);
            }

            if (doc.uses && doc.uses.length > 0) {
                const uses = document.createElement('p');
                uses.style.fontSize = '0.8em';
                uses.style.color = '#888';
                uses.innerHTML = `<strong>USE:</strong> ${doc.uses.join(' ')}`;
                resultDiv.appendChild(uses);
            }

            fragment.appendChild(resultDiv);
        });

        searchResults.appendChild(fragment);
    }

    if (initialQuery) {
        searchInput.value = initialQuery;
        await performSearch(initialQuery);
    }
});
