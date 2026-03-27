const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();

  await page.goto('http://localhost:8082/repos/test-overlay/categories/app-emulation/packages/86box-roms/');
  await page.screenshot({ path: '/home/jules/verification/repo_package2.png', fullPage: true });

  await page.goto('http://localhost:8082/repos/test-overlay/categories/app-emulation/packages/86box-roms/ebuild/5.3/');
  await page.screenshot({ path: '/home/jules/verification/ebuild_details2.png', fullPage: true });

  await browser.close();
})();
