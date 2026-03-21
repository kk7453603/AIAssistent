import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import { QuickAskPage } from "./pages/QuickAskPage";
import "./styles/globals.css";

function getWindowLabel(): string {
  try {
    // __TAURI_INTERNALS__ only exists inside Tauri runtime
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const internals = (window as any).__TAURI_INTERNALS__;
    if (internals?.metadata?.currentWebview?.label) {
      return internals.metadata.currentWebview.label;
    }
  } catch {
    // not in Tauri
  }
  return "main";
}

function Root() {
  const label = getWindowLabel();

  if (label === "quick-ask") {
    return <QuickAskPage />;
  }

  return <App />;
}

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <Root />
  </React.StrictMode>,
);
