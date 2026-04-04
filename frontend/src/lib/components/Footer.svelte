<script lang="ts">
    import { _ } from 'svelte-i18n';

    function handleLinkClick(e: MouseEvent, path: string) {
        e.preventDefault();
        window.history.pushState({}, "", path);
        window.dispatchEvent(new PopStateEvent("popstate"));
    }
</script>

<footer>
    <div class="footer-content">
        <nav class="footer-links">
            <a href="/privacy" on:click={(e) => handleLinkClick(e, "/privacy")}
                >{$_('footer.privacy')}</a
            >
        </nav>
    </div>
</footer>

<style>
    footer {
        background: var(--bg-secondary);
        border-top: 1px solid var(--border);
        padding: 1.5rem;
    }

    .footer-content {
        display: flex;
        flex-direction: column;
        align-items: center;
        gap: 0.75rem;
        max-width: 87.5rem;
        margin: 0 auto;
    }

    .footer-links {
        display: flex;
        gap: 1.5rem;
    }

    .footer-links a {
        color: var(--text-secondary);
        font-size: 0.875rem;
        text-decoration: none;
    }

    .footer-links a:hover {
        color: var(--text-primary);
    }

    @media (min-width: 768px) {
        .footer-content {
            flex-direction: row;
            justify-content: space-between;
        }
    }
</style>
