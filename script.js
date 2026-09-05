const header = document.querySelector("[data-header]");
const progress = document.querySelector("[data-progress]");
const menuButton = document.querySelector("[data-menu-button]");
const mobileMenu = document.querySelector("[data-mobile-menu]");
const heroMedia = document.querySelector("[data-parallax]");
const hero = document.querySelector(".hero");
const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

/* Nav links paired with their section, so the header can show where you are. */
const navLinks = [...document.querySelectorAll("[data-nav-link]")];
const navTargets = [...new Set(navLinks.map((link) => link.hash))]
  .map((hash) => ({ hash, section: document.querySelector(hash) }))
  .filter((target) => target.section);

const setActiveSection = (hash) => {
  navLinks.forEach((link) => {
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
  revealObserver.observe(element);
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

const videoDialog = document.querySelector("[data-video-dialog]");
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
