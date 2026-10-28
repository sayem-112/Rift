import { createBrowserRouter } from "react-router";
import { SignInPage } from "../features/auth/SignInPage";
import { LibraryPage } from "../features/library/LibraryPage";
import { ReaderPlaceholder } from "../features/reader/ReaderPlaceholder";
import { SettingsPage } from "../features/settings/SettingsPage";
import { AppShell } from "./AppShell";
import { NotFound } from "./NotFound";

export const router = createBrowserRouter([
  // Loaded on demand: the landing page and its animation library stay out of the app bundle.
  { path: "/welcome", lazy: async () => ({ Component: (await import("../features/landing/LandingPage")).default }) },
  { path: "/signin", element: <SignInPage /> },
  { path: "/read/:bookId", element: <ReaderPlaceholder /> },
  {
    element: <AppShell />,
    children: [
      { index: true, element: <LibraryPage filter="all" /> },
      { path: "reading", element: <LibraryPage filter="reading" /> },
      { path: "downloaded", element: <LibraryPage filter="downloaded" /> },
      { path: "finished", element: <LibraryPage filter="finished" /> },
      { path: "settings", element: <SettingsPage /> },
    ],
  },
  { path: "*", element: <NotFound /> },
]);
