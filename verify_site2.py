from playwright.sync_api import sync_playwright

def verify_feature(page):
    page.goto("http://localhost:8000")
    page.wait_for_timeout(500)

    # Click Profiles from Dashboard
    page.get_by_role("link", name="Profiles", exact=True).click()
    page.wait_for_timeout(500)

    # Click specific profile
    page.get_by_role("link", name="default/linux/amd64/23.0/systemd").click()
    page.wait_for_timeout(500)

    # Click parent profile
    page.get_by_role("link", name="default/linux/amd64/23.0", exact=True).click()
    page.wait_for_timeout(1000)

    page.screenshot(path="/home/jules/verification/verification.png")

with sync_playwright() as p:
    browser = p.chromium.launch(headless=True)
    context = browser.new_context(record_video_dir="/home/jules/verification/video")
    page = context.new_page()
    try:
        verify_feature(page)
    finally:
        context.close()
        browser.close()
