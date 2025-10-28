# JavaScript Extraction Cookbook

Here are example JavaScript snippets for extracting common types of information from a web page, using traumwind.de as an example. The snippets below include the exact code and representative output so you can reuse them when driving pages through MCP.

---

### 1. Extract All Headings (h1-h6)

**Code used:**
```js
(() => {
  // Get all headings (h1-h6)
  const headings = Array.from(document.querySelectorAll('h1, h2, h3, h4, h5, h6')).map(h => ({
    tag: h.tagName,
    text: h.textContent.trim()
  }));
  return headings;
})()
```

**Returns:**
```json
[
  {"tag": "H2", "text": "Startseite"},
  {"tag": "H3", "text": "Osnabrück"},
  {"tag": "H3", "text": "IHK Zertifikat"},
  {"tag": "H3", "text": "Oldenburg"},
  {"tag": "H3", "text": "101010"},
  {"tag": "H3", "text": "40"},
  {"tag": "H3", "text": "Designing a Tarot Deck in Tinderbox (Frühjahr 2008)"},
  {"tag": "H3", "text": "The Joy of Noise / Rauschkulissen (Sommer 2006)"},
  {"tag": "H3", "text": "Ich bin ein Broadbandmechanic (Juni '06)"},
  {"tag": "H3", "text": "Tindertraum weblog"},
  {"tag": "H3", "text": "Warhammer 40000 Bücher Liste und Besprechungen (Sommer 2005)"},
  {"tag": "H3", "text": "Prinz Gregor (21 April 2005)"},
  {"tag": "H3", "text": "Seminare und Workshops (Februar 2005)"},
  {"tag": "H3", "text": "Making of Devine Awakening (9. Juni 2004)"},
  {"tag": "H3", "text": "mein eigener Boss (Januar 2004)"},
  {"tag": "H3", "text": "ein lang fälliges Redesign (7. Juni 2002)"},
  {"tag": "H3", "text": "Photos (12. Oktober 2001)"},
  {"tag": "H3", "text": "Der Fang (9. Oktober 2001)"},
  {"tag": "H3", "text": "Psion Scripting Links"},
  {"tag": "H3", "text": "Das Rote Band"},
  {"tag": "H3", "text": "Hello 100world (1. November 2000)"},
  {"tag": "H3", "text": "24 Stunden im (wahren) Leben (23. Sept. 2000)"},
  {"tag": "H3", "text": "Martin auf Englisch (28. Feb. 2000)"},
  {"tag": "H3", "text": "Ich habe unter http://traumwind.EditThisPage.com ein 'Web-Log' gestartet. Dort spreche ich Englisch."},
  {"tag": "H3", "text": "Außerdem habe ich mir hier mal 'nen Tag Zeit genommen, um auch diese Seiten etwas auf 'Vordermann' zu bringen."},
  {"tag": "H3", "text": "Und schon wieder was Neues... (11. Januar 2000)"},
  {"tag": "H3", "text": "Frohe Feiertage! (25. Dezember 1999)"},
  {"tag": "H3", "text": "Onwards! (18. Juli 1999)"},
  {"tag": "H3", "text": "Na endlich! (8. Juni 1999)"},
  {"tag": "H3", "text": "Hallo! (18. April 1999)"},
  {"tag": "H3", "text": "Similar"}
]
```

---

### 2. Extract All Anchor Links (<a> tags)

**Code used:**
```js
(() => {
  // Get all anchor links (hrefs)
  const links = Array.from(document.querySelectorAll('a')).map(a => ({
    href: a.href,
    text: a.textContent.trim()
  }));
  return links;
})()
```

**Returns:** (truncated for brevity)
```json
[
  {"href": "https://traumwind.de/geschichten/", "text": "Geschichten"},
  {"href": "https://traumwind.de/kunst/", "text": "Kunst"},
  {"href": "https://traumwind.de/music/", "text": "Musik"},
  {"href": "https://traumwind.de/computer/", "text": "Technik"},
  ...
  {"href": "https://traumwind.de/Traumtank/?/Traumtank", "text": "Traumtank"}
]
```

---

### 3. Extract the Page Title

**Code used:**
```js
(() => {
  // Get the page title
  return document.title;
})()
```

**Returns:**
```
"Traumwind - Startseite"
```

---

All scripts were run with `showErrors: true`. No errors occurred in these cases, but if there had been, the tool would have included the error message in the response.

If you want to extract other details (meta tags, images, scripts, etc.), let me know, and I can generate more cookbook examples!
