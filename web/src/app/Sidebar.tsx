import { BookOpen, Books, CheckCircle, DownloadSimple, GearSix, HardDrives, MagnifyingGlass, type Icon } from "@phosphor-icons/react";
import { NavLink } from "react-router";
import { Glass } from "../components/glass/Glass";
import { openPalette } from "../components/palette/paletteStore";
import { ProgressBar } from "../components/Progress";
import { Wordmark } from "../components/RiftMark";
import { FILTERS, LIBRARY_QUOTA, matchesFilter, usedBytes, type FilterId } from "../features/library/library";
import { useLibrary } from "../features/library/libraryStore";
import { formatBytes } from "../lib/format";
import { modKeyLabel } from "../lib/platform";
import styles from "./Sidebar.module.css";

export const FILTER_ICONS: Record<FilterId, Icon> = {
  all: Books,
  reading: BookOpen,
  downloaded: DownloadSimple,
  finished: CheckCircle,
};

export function Sidebar() {
  const { books } = useLibrary();
  const used = usedBytes(books);

  return (
    <aside className={styles.sidebar}>
      <div className={styles.brand}>
        <Wordmark size={21} />
      </div>

      <button type="button" className={styles.search} onClick={openPalette}>
        <MagnifyingGlass size={17} aria-hidden="true" />
        <span>Search</span>
        <kbd className={styles.kbd} aria-label={`${modKeyLabel} K`}>
          {modKeyLabel} K
        </kbd>
      </button>

      <nav aria-label="Library" className={styles.nav}>
        {FILTERS.map((filter) => {
          const FilterIcon = FILTER_ICONS[filter.id];
          const count = books.filter((b) => matchesFilter(b, filter.id)).length;
          return (
            <NavLink key={filter.id} to={filter.path} end viewTransition className={styles.link}>
              {({ isActive }) => (
                <>
                  <FilterIcon size={19} weight={isActive ? "fill" : "regular"} className={styles.linkIcon} aria-hidden="true" />
                  <span className={styles.linkLabel}>{filter.label}</span>
                  <span className={`${styles.count} tabular`} aria-label={`${count} books`}>
                    {count}
                  </span>
                </>
              )}
            </NavLink>
          );
        })}
      </nav>

      <div className={styles.footer}>
        <NavLink to="/settings#storage" className={styles.storageLink} aria-label={`Storage: ${formatBytes(used)} of ${formatBytes(LIBRARY_QUOTA)} used`}>
          <Glass radius={18} refraction={0} tint="var(--ctp-base)" className={styles.storage}>
            <span className={styles.storageHead}>
              <HardDrives size={15} weight="fill" aria-hidden="true" className={styles.storageIcon} />
              <span className={styles.storageLabel}>Storage</span>
              <span className={`${styles.storageText} tabular`}>
                {formatBytes(used)}
                <span className={styles.storageOf}> / {formatBytes(LIBRARY_QUOTA)}</span>
              </span>
            </span>
            <ProgressBar value={used / LIBRARY_QUOTA} label="Storage used" size="md" />
          </Glass>
        </NavLink>

        <NavLink to="/settings" className={styles.link}>
          {({ isActive }) => (
            <>
              <GearSix size={19} weight={isActive ? "fill" : "regular"} className={styles.linkIcon} aria-hidden="true" />
              <span className={styles.linkLabel}>Settings</span>
            </>
          )}
        </NavLink>
      </div>
    </aside>
  );
}
