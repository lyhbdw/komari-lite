// routes.js
import { lazy } from "react";
import type { RouteObject } from "react-router-dom";
import React from "react";
import { Navigate } from "react-router-dom";

const Index = lazy(() => import("./pages/Index"));
const AdminLayout = lazy(() => import("./pages/admin/_layout"));
const Admin = lazy(() => import("./pages/admin"));
const NotFound = lazy(() => import("./pages/404"));

export const routes: RouteObject[] = [
  {
    path: "/",
    element: React.createElement(lazy(() => import("./pages/_layout"))),
    children: [
      { index: true, element: React.createElement(Index) },
      {
        path: "instance/:uuid",
        element: React.createElement(lazy(() => import("./pages/instance"))),
      },
    ],
  },

  {
    path: "/admin",
    element: React.createElement(AdminLayout),
    children: [
      { index: true, element: React.createElement(Navigate, { to: "/admin/servers", replace: true }) },
      {
        path: "servers",
        element: React.createElement(Admin),
      },

      {
        path: "sessions",
        element: React.createElement(
          lazy(() => import("./pages/admin/sessions"))
        ),
      },
      {
        path: "account",
        element: React.createElement(
          lazy(() => import("./pages/admin/account"))
        ),
      },
      {
        path: "settings",
        element: React.createElement(
          lazy(() => import("./pages/admin/settings/_layout"))
        ),
        children: [
          {
            index: true,
            element: React.createElement(Navigate, {
              to: "/admin/settings/site",
              replace: true,
            }),
          },
          {
            path: "site",
            element: React.createElement(
              lazy(() => import("./pages/admin/settings/site"))
            ),
          },


          {
            path: "notification",
            element: React.createElement(Navigate, {
              to: "/admin/notification/channels",
              replace: true,
            }),
          },
          {
            path: "general",
            element: React.createElement(
              lazy(() => import("./pages/admin/settings/general"))
            ),
          },


        ],
      },
      {
        path: "notification",
        children: [
          {
            index: true,
            element: React.createElement(Navigate, {
              to: "/admin/notification/channels",
              replace: true,
            }),
          },
          {
            path: "channels",
            element: React.createElement(
              lazy(() => import("./pages/admin/settings/_layout"))
            ),
            children: [
              {
                index: true,
                element: React.createElement(
                  lazy(() => import("./pages/admin/notification/channels"))
                ),
              },
            ],
          },
          {
            path: "offline",
            element: React.createElement(
              lazy(() => import("./pages/admin/notification/offline"))
            ),
          },

          {
            path: "general",
            element: React.createElement(
              lazy(() => import("./pages/admin/notification/general"))
            ),
          },
        ],
      },
      {
        path: "ping",
        element: React.createElement(
          lazy(() => import("./pages/admin/pingTask"))
        ),
      },
      {
        path: "logs",
        element: React.createElement(lazy(() => import("./pages/admin/log"))),
      },
      {
        path: "themes",
        element: React.createElement(lazy(() => import("./pages/admin/themes"))),
      },
      {
        path: "themes/settings",
        element: React.createElement(
          lazy(() => import("./pages/admin/theme_settings")),
        ),
      },
    ],
  },

  // Catch-all 404 route
  { path: "*", element: React.createElement(NotFound) },
];
