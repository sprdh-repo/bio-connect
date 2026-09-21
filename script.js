const header = document.querySelector("[data-header]");
const progress = document.querySelector("[data-progress]");
const menuButton = document.querySelector("[data-menu-button]");
const mobileMenu = document.querySelector("[data-mobile-menu]");
const heroMedia = document.querySelector("[data-parallax]");
const hero = document.querySelector(".hero");
const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

const programme = document.querySelector("#programme");
const registration = document.querySelector("#registration");
if (programme && registration) programme.after(registration);

/* Intro curtain. The root class is set in the page head, before first paint -
   here we only run the exit and tell the rest of the page when to start. */
const intro = document.querySelector("[data-intro]");
const introRunning = document.documentElement.classList.contains("intro-on") && intro;

const introDone = new Promise((resolve) => {
  if (!introRunning) {
    resolve();
    return;
  }

  const introCount = intro.querySelector("[data-intro-count]");
  const countStartedAt = performance.now();

  const countUp = (now) => {
    const progress = Math.min((now - countStartedAt) / 1550, 1);
    introCount.textContent = String(Math.round(progress * 100)).padStart(2, "0");
    if (progress < 1) requestAnimationFrame(countUp);
  };
  requestAnimationFrame(countUp);

  let settled = false;
  const endIntro = () => {
    if (settled) return;
    settled = true;
    intro.classList.add("is-done");
    document.documentElement.classList.remove("intro-on");
    /* Wait for the panel's own lift - the fades on its children bubble up here
       too, and acting on those would cut the curtain off half raised. */
    intro.addEventListener("transitionend", (event) => {
      if (event.target === intro && event.propertyName === "transform") intro.remove();
    });
    setTimeout(resolve, 420);
  };

  /* Nobody should be held hostage by an animation - any input skips it. */
  window.addEventListener("keydown", endIntro, { once: true });
  intro.addEventListener("pointerdown", endIntro, { once: true });
  window.addEventListener("wheel", endIntro, { once: true, passive: true });
  setTimeout(endIntro, 2150);
});

/* The conclave opens on the morning of 8 October 2026, India time. */
const EVENT_START = new Date("2026-10-08T09:00:00+05:30");
const countdown = document.querySelector("[data-countdown]");

if (countdown) {
  const units = [
    { node: countdown.querySelector('[data-unit="days"]'), per: 86400000 },
    { node: countdown.querySelector('[data-unit="hours"]'), per: 3600000, of: 24 },
    { node: countdown.querySelector('[data-unit="minutes"]'), per: 60000, of: 60 },
    { node: countdown.querySelector('[data-unit="seconds"]'), per: 1000, of: 60 },
  ];
  const title = countdown.querySelector("[data-countdown-title]");
  const lede = countdown.querySelector("[data-countdown-lede]");

  const render = () => {
    const remaining = Math.max(EVENT_START - Date.now(), 0);

    units.forEach(({ node, per, of }) => {
      const value = of ? Math.floor(remaining / per) % of : Math.floor(remaining / per);
      const next = String(value).padStart(2, "0");
      if (node.textContent === next) return;
      node.textContent = next;
      if (reduceMotion) return;
      const unit = node.parentElement;
      unit.classList.add("is-tick");
      unit.addEventListener("animationend", () => unit.classList.remove("is-tick"), { once: true });
    });

    if (remaining > 0) return true;
    /* The heading carries the date, so "opens on" has to go with it - what is
       left reads "Bio Connect 4.0 is live." over the venue. */
    title.textContent = title.dataset.titleLive;
    lede?.remove();
    return false;
  };

  if (render()) {
    const ticker = setInterval(() => render() || clearInterval(ticker), 1000);
  }
}

/* Arriving from another page on a link like index.html#highlights, the browser
   would smooth-scroll the whole way down after load. Land there directly, then
   hand smooth scrolling back to in-page clicks. */
if (location.hash) {
  const root = document.documentElement;
  root.style.scrollBehavior = "auto";
  window.addEventListener("load", () => {
    document.getElementById(location.hash.slice(1))?.scrollIntoView();
    requestAnimationFrame(() => root.style.removeProperty("scroll-behavior"));
  });
}

/* Nav links paired with their section, so the header can show where you are.
   Only links into this page take part - links to another page carry a static
   aria-current="page" in the markup, which scroll-spy must leave alone. */
const spyLinks = [...document.querySelectorAll("[data-nav-link]")].filter(
  (link) => link.hash && link.pathname === location.pathname && document.querySelector(link.hash),
);
const navTargets = [...new Set(spyLinks.map((link) => link.hash))].map((hash) => ({
  hash,
  section: document.querySelector(hash),
}));

const setActiveSection = (hash) => {
  spyLinks.forEach((link) => {
    if (link.hash === hash) link.setAttribute("aria-current", "true");
    else link.removeAttribute("aria-current");
  });
};

const onScroll = () => {
  const scrolled = window.scrollY;
  header.classList.toggle("scrolled", scrolled > 24);

  const scrollable = document.documentElement.scrollHeight - window.innerHeight;
  progress.style.setProperty("--progress", scrollable > 0 ? Math.min(scrolled / scrollable, 1) : 0);

  if (heroMedia && !reduceMotion) {
    const offset = Math.min(scrolled, hero.offsetHeight) * 0.12;
    heroMedia.style.setProperty("--parallax", `${offset}px`);
  }

  if (!navTargets.length) return;

  /* Active nav = last section whose top has passed under the header. The final
     target (the footer) sits below the deepest scroll position, so the bottom of
     the page counts as reaching it. */
  const line = scrolled + 140;
  let active = null;
  navTargets.forEach((target) => {
    if (target.section.offsetTop <= line) active = target.hash;
  });
  if (scrolled + window.innerHeight >= document.documentElement.scrollHeight - 2) {
    active = navTargets[navTargets.length - 1].hash;
  }
  setActiveSection(active);
};

let scrollQueued = false;
window.addEventListener(
  "scroll",
  () => {
    if (scrollQueued) return;
    scrollQueued = true;
    requestAnimationFrame(() => {
      scrollQueued = false;
      onScroll();
    });
  },
  { passive: true },
);
window.addEventListener("resize", onScroll, { passive: true });
onScroll();

const setMenu = (open) => {
  menuButton.setAttribute("aria-expanded", String(open));
  mobileMenu.classList.toggle("open", open);
  mobileMenu.inert = !open;
  document.body.classList.toggle("menu-open", open);
  /* Keep focus inside the panel while it covers the page. */
  document.querySelectorAll("main, .site-footer").forEach((region) => {
    region.inert = open;
  });
  /* Wait a frame: the panel is visibility:hidden until the open class lands. */
  if (open) requestAnimationFrame(() => mobileMenu.querySelector("a").focus());
};

const closeMenu = ({ restoreFocus = false } = {}) => {
  if (menuButton.getAttribute("aria-expanded") !== "true") return;
  setMenu(false);
  if (restoreFocus) menuButton.focus();
};

menuButton.addEventListener("click", () => setMenu(menuButton.getAttribute("aria-expanded") !== "true"));
mobileMenu.querySelectorAll("a").forEach((link) => link.addEventListener("click", () => closeMenu()));
window.addEventListener("keydown", (event) => event.key === "Escape" && closeMenu({ restoreFocus: true }));
/* The panel only exists below 900px - never leave the page inert on resize. */
window.matchMedia("(min-width: 901px)").addEventListener("change", (event) => event.matches && closeMenu());

const revealObserver = new IntersectionObserver(
  (entries) => {
    entries.forEach((entry) => {
      if (entry.isIntersecting) {
        entry.target.classList.add("visible");
        revealObserver.unobserve(entry.target);
      }
    });
  },
  { threshold: 0.12 },
);

const heroReveals = [...document.querySelectorAll(".hero-content .reveal")];
document.querySelectorAll(".reveal").forEach((element, index) => {
  if (!reduceMotion) {
    /* The hero reads as one deliberate sequence; everything else stays quick. */
    const heroIndex = heroReveals.indexOf(element);
    element.style.transitionDelay =
      heroIndex >= 0 ? `${120 + heroIndex * 110}ms` : `${Math.min(index % 4, 2) * 70}ms`;
  }
  /* Behind the curtain the hero would reveal to nobody - wait for the lift. */
  introDone.then(() => revealObserver.observe(element));
});

const numberFormatter = new Intl.NumberFormat("en-IN");
const countObserver = new IntersectionObserver(
  (entries) => {
    entries.forEach((entry) => {
      if (!entry.isIntersecting) return;
      const target = Number(entry.target.dataset.count);
      const duration = reduceMotion ? 0 : 1200;
      const startedAt = performance.now();

      const tick = (now) => {
        const progress = duration === 0 ? 1 : Math.min((now - startedAt) / duration, 1);
        const eased = 1 - Math.pow(1 - progress, 3);
        entry.target.textContent = numberFormatter.format(Math.round(target * eased));
        if (progress < 1) requestAnimationFrame(tick);
      };

      requestAnimationFrame(tick);
      countObserver.unobserve(entry.target);
    });
  },
  { threshold: 0.7 },
);

document.querySelectorAll("[data-count]").forEach((counter) => countObserver.observe(counter));

/* Add-to-calendar disclosure. Plain links behind a toggle - the .ics download
   covers every client the two deep links don't. */
const calendar = document.querySelector("[data-calendar]");

if (calendar) {
  const calendarButton = calendar.querySelector("[data-calendar-button]");
  const calendarMenu = calendar.querySelector("[data-calendar-menu]");

  const setCalendar = (open) => {
    calendarButton.setAttribute("aria-expanded", String(open));
    calendarMenu.hidden = !open;
  };

  calendarButton.addEventListener("click", () =>
    setCalendar(calendarButton.getAttribute("aria-expanded") !== "true"),
  );
  calendarMenu.querySelectorAll("a").forEach((link) => link.addEventListener("click", () => setCalendar(false)));
  document.addEventListener("click", (event) => calendar.contains(event.target) || setCalendar(false));
  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape" || calendarMenu.hidden) return;
    setCalendar(false);
    calendarButton.focus();
  });
}

/* The highlights player only exists on the home page. */
const videoDialog = document.querySelector("[data-video-dialog]");

if (videoDialog) {
  const dialogFrame = document.querySelector("[data-dialog-frame]");

  const closeVideo = () => {
    videoDialog.close();
    dialogFrame.replaceChildren();
  };

  document.querySelectorAll("[data-video-url]").forEach((trigger) => {
    trigger.addEventListener("click", () => {
      const isNativeVideo = trigger.dataset.videoType === "video";
      const player = document.createElement(isNativeVideo ? "video" : "iframe");
      player.src = trigger.dataset.videoUrl;
      player.title = trigger.dataset.videoTitle;

      if (isNativeVideo) {
        player.controls = true;
        player.autoplay = true;
      } else {
        player.allow = "accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share";
        player.allowFullscreen = true;
      }

      dialogFrame.replaceChildren(player);
      videoDialog.showModal();
    });
  });

  document.querySelector("[data-dialog-close]").addEventListener("click", closeVideo);
  videoDialog.addEventListener("click", (event) => event.target === videoDialog && closeVideo());
  videoDialog.addEventListener("cancel", () => dialogFrame.replaceChildren());
}

/* Speaker search only exists on the speakers page. Without script every card
   simply shows, so the field stays hidden until it can work. */
const speakerGrid = document.querySelector("[data-speaker-grid]");

if (speakerGrid) {
  const search = document.querySelector("[data-speaker-search]");
  const input = search.querySelector("[data-speaker-query]");
  const empty = document.querySelector("[data-speaker-empty]");
  const emptyQuery = empty.querySelector("[data-speaker-empty-query]");

  /* Letters and digits only, so "tp singh", "T.P. Singh" and "t p singh" all
     find Prof. T. P. Singh. */
  const compact = (text) =>
    text.normalize("NFD").replace(/[\u0300-\u036f]/g, "").toLowerCase().replace(/[^a-z0-9]+/g, "");
  /* Name, role and organisation only - not the screen-reader text on the
     LinkedIn and profile controls, or "linkedin" would match every card. */
  const speakers = [...speakerGrid.querySelectorAll(".speaker")].map((card) => ({
    card,
    text: compact(
      [...card.querySelectorAll(".speaker-name, .speaker-role, .speaker-org")].map((node) => node.textContent).join(" "),
    ),
  }));

  const filterSpeakers = () => {
    const query = input.value.trim();
    const terms = query.split(/\s+/).map(compact).filter(Boolean);
    let shown = 0;

    speakers.forEach(({ card, text }) => {
      const match = terms.every((term) => text.includes(term));
      card.hidden = !match;
      if (match) shown += 1;
    });

    emptyQuery.textContent = `“${query}”`;
    empty.hidden = shown > 0;
  };

  const clearSpeakers = () => {
    input.value = "";
    filterSpeakers();
  };

  input.addEventListener("input", filterSpeakers);
  input.addEventListener("keydown", (event) => {
    if (event.key !== "Escape" || !input.value) return;
    event.preventDefault();
    clearSpeakers();
  });
  empty.querySelector("[data-speaker-clear]").addEventListener("click", () => {
    clearSpeakers();
    input.focus();
  });

  search.hidden = false;
  /* A back/forward visit can restore a typed query - honour it. */
  if (input.value) filterSpeakers();

  /* Speaker profiles. Each card gets a "View profile" button stretched over the
     whole card; the LinkedIn link sits above it, so both stay reachable. The
     dialog steps through the cards the current search leaves visible. */
  const dialog = document.querySelector("[data-speaker-dialog]");
  const portrait = dialog.querySelector("[data-speaker-dialog-portrait]");
  const dialogName = dialog.querySelector("[data-speaker-dialog-name]");
  const dialogRole = dialog.querySelector("[data-speaker-dialog-role]");
  const dialogOrg = dialog.querySelector("[data-speaker-dialog-org]");
  const dialogLinkedin = dialog.querySelector("[data-speaker-dialog-linkedin]");
  const dialogCount = dialog.querySelector("[data-speaker-dialog-count]");
  const stepButtons = dialog.querySelectorAll("[data-speaker-dialog-step]");
  let current = null;

  const showSpeaker = (card) => {
    current = card;
    const name = card.querySelector(".speaker-name").textContent;
    const linkedin = card.querySelector(".speaker-linkedin");

    portrait.replaceChildren(card.querySelector(".speaker-portrait").cloneNode(true));
    dialogName.textContent = name;
    dialogRole.textContent = card.querySelector(".speaker-role").textContent;
    dialogOrg.textContent = card.querySelector(".speaker-org").textContent;
    /* Hiding the focused LinkedIn button would drop focus out of the dialog. */
    if (!linkedin && document.activeElement === dialogLinkedin) dialog.querySelector("[data-speaker-dialog-close]").focus();
    dialogLinkedin.hidden = !linkedin;
    if (linkedin) {
      dialogLinkedin.href = linkedin.href;
      dialogLinkedin.setAttribute("aria-label", `${name} on LinkedIn (opens in a new tab)`);
    }

    const visible = speakers.filter(({ card: item }) => !item.hidden).map(({ card: item }) => item);
    dialogCount.textContent = `${String(visible.indexOf(card) + 1).padStart(2, "0")} / ${String(visible.length).padStart(2, "0")}`;
    stepButtons.forEach((button) => (button.hidden = visible.length < 2));
  };

  const stepSpeaker = (step) => {
    const visible = speakers.filter(({ card }) => !card.hidden).map(({ card }) => card);
    if (visible.length < 2) return;
    const index = visible.indexOf(current);
    showSpeaker(visible[(index + step + visible.length) % visible.length]);
  };

  const closeProfile = () => dialog.close();

  speakers.forEach(({ card }) => {
    const name = card.querySelector(".speaker-name").textContent;
    const open = document.createElement("button");
    open.type = "button";
    open.className = "speaker-open";
    open.innerHTML = `View profile<span class="sr-only">: </span><span class="speaker-open-arrow" aria-hidden="true">&rarr;</span>`;
    open.querySelector(".sr-only").append(name);
    open.setAttribute("aria-haspopup", "dialog");
    open.addEventListener("click", () => {
      showSpeaker(card);
      dialog.showModal();
      document.body.classList.add("dialog-open");
    });

    let actions = card.querySelector(".speaker-actions");
    if (!actions) {
      actions = document.createElement("div");
      actions.className = "speaker-actions";
      card.querySelector(".speaker-meta").append(actions);
    }
    actions.append(open);
  });

  stepButtons.forEach((button) =>
    button.addEventListener("click", () => stepSpeaker(Number(button.dataset.speakerDialogStep))),
  );
  dialog.querySelector("[data-speaker-dialog-close]").addEventListener("click", closeProfile);
  /* A click on the backdrop lands on the dialog element itself. */
  dialog.addEventListener("click", (event) => event.target === dialog && closeProfile());
  document.addEventListener("keydown", (event) => {
    if (!dialog.open) return;
    const step = { ArrowLeft: -1, ArrowRight: 1 }[event.key];
    if (!step) return;
    event.preventDefault();
    stepSpeaker(step);
  });
  /* Back on the page, focus returns to the card whose profile was last shown -
     which may not be the one that opened the dialog, after stepping through. */
  dialog.addEventListener("close", () => {
    document.body.classList.remove("dialog-open");
    current?.querySelector(".speaker-open").focus({ preventScroll: true });
    current?.scrollIntoView({ block: "nearest", behavior: reduceMotion ? "auto" : "smooth" });
  });
}

/* Registration tabs: one panel at a time, with roving focus across the tablist. */
const regTabs = [...document.querySelectorAll("[data-reg-tab]")];

const selectRegTab = (tab, { focus = false } = {}) => {
  regTabs.forEach((item) => {
    const selected = item === tab;
    item.setAttribute("aria-selected", String(selected));
    item.tabIndex = selected ? 0 : -1;
    document.getElementById(item.getAttribute("aria-controls")).hidden = !selected;
  });
  if (focus) tab.focus();
};

regTabs.forEach((tab, index) => {
  tab.addEventListener("click", () => selectRegTab(tab));
  tab.addEventListener("keydown", (event) => {
    const step = { ArrowRight: 1, ArrowLeft: -1, Home: -index, End: regTabs.length - 1 - index }[event.key];
    if (step === undefined) return;
    event.preventDefault();
    selectRegTab(regTabs[(index + step + regTabs.length) % regTabs.length], { focus: true });
  });
});

/* Keep the payment link usable when shared or opened in a fresh visit. */
const revealSponsorshipPayment = () => {
  if (location.hash !== "#sponsorship-payment") return;
  const sponsorTab = regTabs.find((tab) => tab.id === "reg-tab-sponsors");
  if (!sponsorTab) return;
  selectRegTab(sponsorTab);
  document.getElementById("sponsorship-payment").scrollIntoView();
};
window.addEventListener("hashchange", revealSponsorshipPayment);
revealSponsorshipPayment();

/* Anonymous, current exhibitor profiles from the registration backend. */
const exhibitorDirectory = document.querySelector('[data-exhibitor-directory]');
if (exhibitorDirectory) {
  const endpoint = 'https://reg.bioconnect.kerala.gov.in/api/v1/public/exhibitors';
  const grid = exhibitorDirectory.querySelector('[data-exhibitor-grid]');
  const state = exhibitorDirectory.querySelector('[data-directory-state]');
  const toolbar = exhibitorDirectory.querySelector('[data-directory-toolbar]');
  const count = exhibitorDirectory.querySelector('[data-directory-count]');
  const query = exhibitorDirectory.querySelector('[data-exhibitor-query]');
  const retry = exhibitorDirectory.querySelector('[data-directory-retry]');
  const clear = exhibitorDirectory.querySelector('[data-directory-clear]');
  let entries = [];
  let cards = [];
  const normalise = (value) => value.normalize('NFKD').replace(/[\u0300-\u036f]/g, '').toLocaleLowerCase();
  const filter = () => {
    const term = normalise(query.value.trim());
    let visible = 0;
    cards.forEach(({ node, text }) => {
      node.hidden = !text.includes(term);
      if (!node.hidden) visible++;
    });
    count.textContent = term ? `${visible} of ${entries.length} exhibitors` : `${entries.length} confirmed exhibitor${entries.length === 1 ? '' : 's'}`;
    state.hidden = visible > 0;
    state.textContent = entries.length === 0 ? 'The exhibitor line-up is being confirmed. Check back soon to discover who is joining the expo.' : visible === 0 ? 'No exhibitors match your search. Try another organisation or area of expertise.' : '';
    clear.hidden = !term;
  };
  const render = () => {
    grid.replaceChildren();
    cards = entries.map((entry, index) => {
      const node = document.createElement('li');
      node.className = 'exhibitor-card';
      const logo = document.createElement('div');
      logo.className = 'exhibitor-logo';
      const fallback = () => {
        const initials = document.createElement('span');
        initials.className = 'exhibitor-monogram';
        initials.setAttribute('aria-hidden', 'true');
        initials.textContent = entry.name.trim().split(/\s+/).slice(0, 2).map(word => Array.from(word)[0] || '').join('');
        logo.replaceChildren(initials);
      };
      fallback();
      if (entry.logo_url) {
        const url = new URL(entry.logo_url, endpoint);
        // Only the dedicated public image route is valid here.
        if (url.origin === new URL(endpoint).origin && /^\/api\/v1\/public\/exhibitors\/logos\/[a-zA-Z0-9_-]+$/.test(url.pathname)) {
          const img = document.createElement('img');
          img.src = url.href;
          img.alt = '';
          img.loading = 'lazy';
          img.decoding = 'async';
          img.addEventListener('error', fallback, { once: true });
          logo.replaceChildren(img);
        }
      }
      const meta = document.createElement('div');
      meta.className = 'exhibitor-meta';
      const label = document.createElement('span');
      label.className = 'detail-label';
      label.textContent = 'Confirmed exhibitor';
      const name = document.createElement('h3');
      name.textContent = entry.name;
      const description = document.createElement('p');
      description.id = `exhibitor-description-${index}`;
      description.className = 'exhibitor-description';
      description.textContent = entry.description;
      meta.append(label, name, description);
      if (entry.description.length > 200) {
        description.classList.add('is-collapsed');
        const expand = document.createElement('button');
        expand.type = 'button';
        expand.className = 'exhibitor-expand';
        expand.setAttribute('aria-expanded', 'false');
        expand.setAttribute('aria-controls', description.id);
        expand.setAttribute('aria-label', `Read company profile: ${entry.name}`);
        expand.textContent = 'Read company profile +';
        expand.addEventListener('click', () => {
          const open = expand.getAttribute('aria-expanded') !== 'true';
          expand.setAttribute('aria-expanded', String(open));
          expand.setAttribute('aria-label', `${open ? 'Close' : 'Read'} company profile: ${entry.name}`);
          expand.textContent = open ? 'Close company profile −' : 'Read company profile +';
          description.classList.toggle('is-collapsed', !open);
        });
        meta.append(expand);
      }
      node.append(logo, meta);
      grid.append(node);
      return { node, text: normalise(`${entry.name} ${entry.description}`) };
    });
    toolbar.hidden = entries.length === 0;
    filter();
  };
  const load = async () => {
    retry.hidden = true;
    state.hidden = false;
    state.textContent = 'Loading the exhibitor line-up…';
    grid.setAttribute('aria-busy', 'true');
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 12000);
    try {
      const response = await fetch(endpoint, { credentials: 'omit', signal: controller.signal, cache: 'no-store' });
      if (!response.ok) throw new Error('Directory unavailable');
      const data = await response.json();
      if (!Array.isArray(data.exhibitors) || !data.exhibitors.every(entry => entry && typeof entry.name === 'string' && typeof entry.description === 'string' && typeof entry.logo_url === 'string')) throw new Error('Invalid directory');
      entries = data.exhibitors;
      render();
    } catch {
      state.textContent = 'We couldn’t load the exhibitor line-up. Please try again in a moment.';
      retry.hidden = false;
    } finally {
      clearTimeout(timeout);
      grid.setAttribute('aria-busy', 'false');
    }
  };
  query.addEventListener('input', filter);
  clear.addEventListener('click', () => { query.value = ''; filter(); query.focus(); });
  retry.addEventListener('click', load);
  load();
}
