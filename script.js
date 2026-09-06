const header = document.querySelector("[data-header]");
const progress = document.querySelector("[data-progress]");
const menuButton = document.querySelector("[data-menu-button]");
const mobileMenu = document.querySelector("[data-mobile-menu]");
const heroMedia = document.querySelector("[data-parallax]");
const hero = document.querySelector(".hero");
const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

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
    title.textContent = title.dataset.titleLive;
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
