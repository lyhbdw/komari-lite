import { createPortal } from "react-dom"
import { Toaster as Sonner, type ToasterProps } from "sonner"

const Toaster = ({ theme, ...props }: ToasterProps) => {
  const resolvedTheme = theme ?? (document.documentElement.classList.contains("dark") ? "dark" : "light")

  return (
    createPortal(
      <div className="km-toaster-portal" data-km-toaster-portal="">
        <Sonner
          theme={resolvedTheme}
          className="toaster group km-ui-toaster"
          style={
            {
              "--normal-bg": "var(--popover)",
              "--normal-text": "var(--popover-foreground)",
              "--normal-border": "var(--border)",
            } as React.CSSProperties
          }
          {...props}
        />
      </div>,
      document.body,
    )
  )
}

export { Toaster }
