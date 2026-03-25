from playwright.sync_api import sync_playwright

def verify_feature(page):
    # Verify Global Dashboard
    page.goto("http://localhost:3000/")
    page.wait_for_timeout(500)

    # Screenshot global dashboard
    page.screenshot(path="/home/jules/verification/screenshot_1.png")
    page.wait_for_timeout(500)

    # Click on the recent news to go to the specific article
    page.get_by_role("link", name="Recent News Article").click()
    page.wait_for_timeout(500)
    page.screenshot(path="/home/jules/verification/screenshot_2.png")
    page.wait_for_timeout(500)

    # Go back to global dashboard
    page.goto("http://localhost:3000/")
    page.wait_for_timeout(500)

    # Go to Global News Dashboard
    page.get_by_role("link", name="More... (Archive)").first.click()
    page.wait_for_timeout(500)
    page.screenshot(path="/home/jules/verification/screenshot_3.png")
    page.wait_for_timeout(500)

    # Go to Repo Dashboard
    page.goto("http://localhost:3000/repos/test_overlay/")
    page.wait_for_timeout(500)
    page.screenshot(path="/home/jules/verification/screenshot_4.png")
    page.wait_for_timeout(500)

    # Go to Repo News Dashboard
    page.get_by_role("link", name="News", exact=True).click()
    page.wait_for_timeout(500)
    page.screenshot(path="/home/jules/verification/screenshot_5.png")
    page.wait_for_timeout(500)

    page.wait_for_timeout(1000)

if __name__ == "__main__":
    import os
    os.makedirs("/home/jules/verification/video", exist_ok=True)
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        context = browser.new_context(record_video_dir="/home/jules/verification/video")
        page = context.new_page()
        try:
            verify_feature(page)
        finally:
            context.close()
            browser.close()
