import i18next from "i18next";
import { initReactI18next } from "react-i18next";
import zh_CN from "./locales/zh_CN.json";

const resources = {
  "zh-CN": {
    translation: zh_CN,
    name: "简体中文",
  },
};

const i18n = i18next;

void i18n
  .use(initReactI18next)
  .init({
    resources,
    lng: "zh-CN",
    fallbackLng: "zh-CN",
    interpolation: {
      escapeValue: false, // React handles XSS
    },
  });

export default i18n;
export { resources };
