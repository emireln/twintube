/* TwinTube desktop i18n (EN / PT) */

const STRINGS = {
  en: {
    show: 'Show TwinTube',
    hide: 'Hide TwinTube',
    quit: 'Quit',
    update_available_title: 'Update available',
    update_available_body: 'TwinTube {version} is ready to download.',
    update_download: 'Download update',
    update_downloading_title: 'Downloading update',
    update_downloading_body: 'TwinTube {version} is downloading in the background.',
    update_progress_label: 'Download progress',
    update_ready_title: 'Update ready',
    update_ready_body: 'TwinTube {version} is downloaded. Restart now to install it.',
    update_restart: 'Restart now',
    update_later: 'Later',
    update_dismiss: 'OK',
    update_error_title: 'Update failed',
    update_error_check: 'Could not check for updates. You can keep using TwinTube.',
    update_error_download: 'Could not download the update. Try again later.'
  },
  pt: {
    show: 'Mostrar TwinTube',
    hide: 'Ocultar TwinTube',
    quit: 'Sair',
    update_available_title: 'Atualização disponível',
    update_available_body: 'O TwinTube {version} está pronto para baixar.',
    update_download: 'Baixar atualização',
    update_downloading_title: 'Baixando atualização',
    update_downloading_body: 'O TwinTube {version} está sendo baixado em segundo plano.',
    update_progress_label: 'Progresso do download',
    update_ready_title: 'Atualização pronta',
    update_ready_body: 'O TwinTube {version} foi baixado. Reinicie agora para instalar.',
    update_restart: 'Reiniciar agora',
    update_later: 'Depois',
    update_dismiss: 'OK',
    update_error_title: 'Falha na atualização',
    update_error_check: 'Não foi possível verificar atualizações. Você pode continuar usando o TwinTube.',
    update_error_download: 'Não foi possível baixar a atualização. Tente novamente mais tarde.'
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

module.exports = { detectLang, t, normalizeLang, STRINGS };
