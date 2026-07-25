/* TwinTube desktop i18n (EN / PT) */

const STRINGS = {
  en: {
    show: 'Show TwinTube',
    hide: 'Hide TwinTube',
    quit: 'Quit',
    update_available_title: 'Update available',
    update_available_body: 'TwinTube {version} is ready to download.',
    update_download: 'Download',
    update_downloading_title: 'Downloading update',
    update_downloading_body: 'Downloading TwinTube {version}…',
    update_ready_title: 'Update ready',
    update_ready_body: 'TwinTube {version} has been downloaded. Restart now to apply the update?',
    update_restart: 'Restart now',
    update_later: 'Later',
    update_error_title: 'Update failed',
    update_error_body: 'Could not check for updates. You can keep using TwinTube.'
  },
  pt: {
    show: 'Mostrar TwinTube',
    hide: 'Ocultar TwinTube',
    quit: 'Sair',
    update_available_title: 'Atualização disponível',
    update_available_body: 'O TwinTube {version} está pronto para baixar.',
    update_download: 'Baixar',
    update_downloading_title: 'Baixando atualização',
    update_downloading_body: 'Baixando TwinTube {version}…',
    update_ready_title: 'Atualização pronta',
    update_ready_body: 'O TwinTube {version} foi baixado. Reiniciar agora para aplicar a atualização?',
    update_restart: 'Reiniciar agora',
    update_later: 'Depois',
    update_error_title: 'Falha na atualização',
    update_error_body: 'Não foi possível verificar atualizações. Você pode continuar usando o TwinTube.'
  }
};

function normalizeLang(raw) {
  const value = String(raw || '').toLowerCase();
  if (value.startsWith('pt')) return 'pt';
  return 'en';
}

function detectLang() {
  try {
    const locale = Intl.DateTimeFormat().resolvedOptions().locale || '';
    return normalizeLang(locale);
  } catch (_) {
    return 'en';
  }
}

function t(key, vars = {}, lang = detectLang()) {
  const dict = STRINGS[lang] || STRINGS.en;
  let text = dict[key] || STRINGS.en[key] || key;
  Object.entries(vars).forEach(([k, v]) => {
    text = text.replace(new RegExp(`\\{${k}\\}`, 'g'), String(v));
  });
  return text;
}

module.exports = { detectLang, t, normalizeLang };
