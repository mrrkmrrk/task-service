// Local UI labels for the pinned Swagger UI distribution. API payloads and
// editable JSON examples are left intact.
(() => {
  const labels = new Map(Object.entries({
    'Authorize': 'Авторизация',
    'Authorization': 'Авторизация',
    'Available authorizations': 'Доступные способы авторизации',
    'Authorized': 'Авторизован',
    'Logout': 'Выйти',
    'Close': 'Закрыть',
    'Value:': 'Значение:',
    'Name:': 'Имя:',
    'In:': 'Расположение:',
    'Servers': 'Серверы',
    'Server variables': 'Переменные сервера',
    'Parameters': 'Параметры',
    'No parameters': 'Нет параметров',
    'Name': 'Имя',
    'Description': 'Описание',
    'required': 'обязательно',
    '* required': '* обязательно',
    'Request body': 'Тело запроса',
    'Request URL': 'URL запроса',
    'Request duration': 'Длительность запроса',
    'Try it out': 'Попробовать',
    'Cancel': 'Отмена',
    'Execute': 'Выполнить',
    'Clear': 'Очистить',
    'Reset': 'Сбросить',
    'Responses': 'Ответы',
    'Server response': 'Ответ сервера',
    'Response body': 'Тело ответа',
    'Response headers': 'Заголовки ответа',
    'Code': 'Код',
    'Details': 'Подробности',
    'Links': 'Ссылки',
    'No links': 'Нет ссылок',
    'Undocumented': 'Не описан в документации',
    'Example Value': 'Пример значения',
    'Example': 'Пример',
    'Examples': 'Примеры',
    'Schema': 'Схема',
    'Schemas': 'Схемы',
    'Model': 'Модель',
    'Models': 'Модели',
    'Media type': 'Тип содержимого',
    'Media Type': 'Тип содержимого',
    'Response content type': 'Тип содержимого ответа',
    'Request content type': 'Тип содержимого запроса',
    'Controls Accept header.': 'Задаёт заголовок Accept.',
    'Controls `Accept` header.': 'Задаёт заголовок Accept.',
    'Controls Content-Type header.': 'Задаёт заголовок Content-Type.',
    'Download': 'Скачать',
    'Download file': 'Скачать файл',
    'Expand operation': 'Развернуть операцию',
    'Collapse operation': 'Свернуть операцию',
    'Expand all': 'Развернуть всё',
    'Collapse all': 'Свернуть всё',
    'Copy to clipboard': 'Копировать',
    'Copy path to clipboard': 'Копировать путь',
    'Apply credentials': 'Применить токен',
    'Remove authorization': 'Удалить авторизацию',
    'Copy': 'Копировать',
    'Copied': 'Скопировано',
    'Filter by tag': 'Фильтр по разделам',
    'Select a definition': 'Выберите спецификацию',
    'Select a spec': 'Выберите спецификацию',
    'Select a server': 'Выберите сервер',
    'Select content type': 'Выберите тип содержимого',
    'Send empty value': 'Отправить пустое значение',
    'Available values': 'Допустимые значения',
    'Default value': 'Значение по умолчанию',
    'Example value': 'Пример значения',
    'No description': 'Нет описания',
    'Loading...': 'Загрузка...',
    'Errors': 'Ошибки',
    'Hide': 'Скрыть',
    'Show': 'Показать',
    'Error': 'Ошибка',
    'Error:': 'Ошибка:',
    'Please correct the following validation errors and try again.': 'Исправьте ошибки проверки данных и повторите запрос.',
    'Required field is not provided': 'Не заполнено обязательное поле',
    'Value must be an integer': 'Значение должно быть целым числом',
    'Value must be a number': 'Значение должно быть числом',
    'Value must be a string': 'Значение должно быть строкой',
    'Value must be a boolean': 'Значение должно быть логическим',
    'auth button': 'Кнопка авторизации',
    'authorization button unlocked': 'Авторизация не настроена',
    'authorization button locked': 'Авторизация настроена'
  }));
  const root = document.getElementById('swagger-ui');
  const ignored = 'script, style, pre, code, textarea, .microlight, .highlight-code';
  function translate(value) {
    const key = value.trim();
    const fieldError = key.match(/^For '([^']+)': (.+)$/);
    if (fieldError) {
      const message = fieldError[2].replace(/\.$/, '');
      return `Поле «${fieldError[1]}»: ${translate(message)}.`;
    }
    const replacement = labels.get(key);
    if (replacement) return value.replace(key, replacement);
    return value
      .replace(/^([\s]*)Available values\s*:/, '$1Допустимые значения:')
      .replace(/^([\s]*)Default value\s*:/, '$1Значение по умолчанию:')
      .replace(/^([\s]*)Example value\s*:/, '$1Пример значения:')
      .replace(/^Value must be greater than or equal to (.+)$/, 'Значение должно быть не меньше $1')
      .replace(/^Value must be less than or equal to (.+)$/, 'Значение должно быть не больше $1');
  }
  function localize() {
    observer.disconnect();
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    let node;
    while ((node = walker.nextNode())) {
      if (!node.parentElement || node.parentElement.closest(ignored)) continue;
      if (node.parentElement.matches('.response-control-media-type__accept-message')) {
        if (node.nodeValue.trim() === 'Controls') node.nodeValue = 'Задаёт заголовок ';
        if (node.nodeValue.trim() === 'header.') node.nodeValue = '.';
      }
      const text = translate(node.nodeValue);
      if (text !== node.nodeValue) node.nodeValue = text;
    }
    root.querySelectorAll('[title], [aria-label], [placeholder]').forEach(element => {
      if (element.closest(ignored)) return;
      for (const attribute of ['title', 'aria-label', 'placeholder']) {
        const value = element.getAttribute(attribute);
        if (value === null) continue;
        const text = translate(value);
        if (text !== value) element.setAttribute(attribute, text);
      }
    });
    observer.observe(root, {subtree:true, childList:true, characterData:true, attributes:true, attributeFilter:['title','aria-label','placeholder']});
  }
  let pending = false;
  const observer = new MutationObserver(() => {
    if (pending) return;
    pending = true;
    requestAnimationFrame(() => { pending = false; localize(); });
  });
  localize();
})();
