import { User } from "@phosphor-icons/react";
import { Link, NavLink } from "react-router";
import { Glass } from "../../components/glass/Glass";
import { Wordmark } from "../../components/RiftMark";
import { FILTERS } from "./library";
import styles from "./MobileHeader.module.css";

/** Phones: a floating glass header with the wordmark, account, and filter chips. */
export function MobileHeader() {
  return (
    <Glass as="header" radius={26} floating refraction={52} className={styles.header}>
      <div className={styles.top}>
        <Wordmark size={20} />
        <Link to="/settings" className={styles.avatar} aria-label="Account and settings">
          <User size={18} weight="bold" aria-hidden="true" />
        </Link>
      </div>
      <nav aria-label="Library" className={styles.chips}>
        {FILTERS.map((filter) => (
          <NavLink
            key={filter.id}
            to={filter.path}
            end
            viewTransition
            className={styles.chip}
          >
            {filter.id === "all" ? "All" : filter.label}
          </NavLink>
        ))}
      </nav>
    </Glass>
  );
}
