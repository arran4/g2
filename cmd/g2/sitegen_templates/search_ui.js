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

    function escapeHTML(str) {
        if (!str) return '';
        return String(str)
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');
    }

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

        const html = results.map(doc => {
            const overlay = escapeHTML(doc.overlay);
            const category = escapeHTML(doc.category);
            const pkg = escapeHTML(doc.package);
            const version = escapeHTML(doc.version);
            const pageUrl = escapeHTML(doc.page_url);
            const description = escapeHTML(doc.description);
            const eapi = escapeHTML(doc.eapi);
            const slot = escapeHTML(doc.slot);
            const mask = escapeHTML(doc.mask);

            let badgesHtml = '';
            const badges = [];
            if (overlay) badges.push(`<span style="background: #e0e0e0; padding: 2px 6px; border-radius: 4px; margin-right: 5px;">${overlay}</span>`);
            if (eapi) badges.push(`EAPI: ${eapi}`);
            if (slot) badges.push(`SLOT: ${slot}`);
            if (mask && mask !== 'none') badges.push(`<span style="color: red;">MASK: ${mask}</span>`);
            badgesHtml = badges.join(' | ');

            let kwHtml = '';
            if (doc.keywords && doc.keywords.length > 0) {
                const escapedKw = doc.keywords.map(escapeHTML).join(' ');
                kwHtml = `<p style="font-size: 0.8em; color: #888;"><strong>Keywords:</strong> ${escapedKw}</p>`;
            }

            let usesHtml = '';
            if (doc.uses && doc.uses.length > 0) {
                const usesLinks = doc.uses.map(use => {
                    const escUse = escapeHTML(use);
                    return `<a href="../uses/${escUse}/">${escUse}</a>`;
                }).join(' ');
                usesHtml = `<p style="font-size: 0.8em; color: #888;"><strong>USE:</strong> ${usesLinks}</p>`;
            }

            return `
<div style="border: 1px solid #eee; padding: 1em; margin-bottom: 1em; border-radius: 5px; background-color: #fcfcfc;">
    <h3 style="margin-top: 0;">
        <a href="../repos/${overlay}/categories/${category}/">${category}</a>/` +
        `<a href="../repos/${overlay}/categories/${category}/packages/${pkg}/">${pkg}</a>-` +
        `<a href="${pageUrl}">${version}</a>
    </h3>
    <p style="font-size: 0.9em; color: #666; margin: 0 0 0.5em 0;">${badgesHtml}</p>
    <p style="margin-bottom: 0.5em;">${description}</p>
    ${kwHtml}
    ${usesHtml}
</div>`;
        }).join('');

        searchResults.innerHTML = html;
    }

    if (initialQuery) {
        searchInput.value = initialQuery;
        await performSearch(initialQuery);
    }
});
