# Image credits

Every photo or video on the site has a row here (DESIGN.md section 8): the proof of where it came from and under which licence. The site shows only "Illustrative photo", because the Pexels, Unsplash and Nappy licences do not require a credit on the page. **The page link is required**; the photographer's name is nice to have. Stock images are never presented as our contributors or clients.

| File | Photographer | Source | Page link | Licence | Downloaded | Used on |
|---|---|---|---|---|---|---|
| home-people-main.jpg → `frontend/content/images/people-linguist.jpg` | Diva Plavalaguna* | Pexels | https://www.pexels.com/photo/5711687/ | Pexels License | 2026-11-03 | Home, The people (large) |
| home-people-engineer.jpg → `frontend/content/images/people-engineer.jpg` | (optional) | Nappy | **Needed** | Nappy licence (to confirm) | 2026-11-03 | Home, The people |
| industry-health.jpg → `frontend/content/images/people-clinician.jpg` | cottonbro studio* | Pexels | https://www.pexels.com/photo/7578814/ | Pexels License | 2026-11-03 | Home, The people (in place of H3) |
| about-meza.jpg | MART PRODUCTION* | Pexels | https://www.pexels.com/photo/7255284/ | Pexels License | 2026-11-03 | About (Stage 6) |
| contributors-detail.jpg | Plann* | Pexels | https://www.pexels.com/photo/4549413/ | Pexels License | 2026-11-03 | Contributors (Stage 6, crop out the laptop name) |

\* Name read from the download file name (Pexels blocks automated page checks). Confirm the display name on the page link.

Originals live only in `docs/content/images/` on your machine (ignored by git; re-download from the page link). `frontend/scripts/prepare-photos.mjs` crops them to the web masters in `frontend/content/images/`, which are committed.

## Not used

| File | Source | Why |
|---|---|---|
| home-people-review.jpg | Pexels 5407205 | Ornate European interior and pill packs; reads as pharmacy stock, not work in an African city. `industry-health.jpg` fits H3 better. |
| contributors-band.jpg | Pexels 889996 | An empty office: C1 asks for several people working. |
| industry-education.jpg | Pexels 9159054 | A posed portrait looking at the camera, arms crossed. |
| industry-telco-bank.jpg | Pexels 8866823 | Black and white, a smiling mouth in close-up; the brief asks for warm colour and no smiles to camera. |
