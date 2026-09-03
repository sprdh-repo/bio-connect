const header = document.querySelector("[data-header]");
const menuButton = document.querySelector("[data-menu-button]");
const mobileMenu = document.querySelector("[data-mobile-menu]");

const updateHeader = () => header.classList.toggle("scrolled", window.scrollY > 24);
updateHeader();
window.addEventListener("scroll", updateHeader, { passive: true });

const closeMenu = () => {
  menuButton.setAttribute("aria-expanded", "false");
  mobileMenu.classList.remove("open");
  document.body.classList.remove("menu-open");
};

menuButton.addEventListener("click", () => {
  const willOpen = menuButton.getAttribute("aria-expanded") !== "true";
  menuButton.setAttribute("aria-expanded", String(willOpen));
  mobileMenu.classList.toggle("open", willOpen);
  document.body.classList.toggle("menu-open", willOpen);
});

mobileMenu.querySelectorAll("a").forEach((link) => link.addEventListener("click", closeMenu));
window.addEventListener("keydown", (event) => event.key === "Escape" && closeMenu());

const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
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

document.querySelectorAll(".reveal").forEach((element, index) => {
  if (!reduceMotion) element.style.transitionDelay = `${Math.min(index % 4, 2) * 70}ms`;
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
