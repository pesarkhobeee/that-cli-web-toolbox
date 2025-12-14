// Example: Scroll to bottom of the page with a smooth animation
// This script waits for dynamic content to load, then scrolls to the bottom

async function scrollToBottom() {
  const delay = ms => new Promise(r => setTimeout(r, ms));
  const step = 500; // pixels per scroll
  const maxScrolls = 10; // limit to prevent infinite scrolling
  let scrollCount = 0;

  while (window.scrollY + window.innerHeight < document.body.scrollHeight && scrollCount < maxScrolls) {
    window.scrollBy(0, step);
    scrollCount++;
    console.log('Scroll', scrollCount, 'of', maxScrolls, '- position:', window.scrollY);
    await delay(300);
  }

  console.log('Done scrolling after', scrollCount, 'steps');
  // Final wait for any remaining content
  await delay(2000);
}
await scrollToBottom();
