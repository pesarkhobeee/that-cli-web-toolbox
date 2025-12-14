// Scroll to bottom multiple times to trigger lazy loading
let h = 0, i = 0;
const maxSteps = 5; // Limit iterations
while (document.body.scrollHeight !== h && i < maxSteps) {
  h = document.body.scrollHeight;
  console.log('Step', ++i, 'height:', h);
  window.scrollTo(0, h);
  await new Promise(r => setTimeout(r, 2000));
}
console.log('Scroll complete, final height:', document.body.scrollHeight);
