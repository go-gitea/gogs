---
date: "2017-04-15T14:56:00+02:00"
title: "Настройка Gitea"
slug: "customizing-gitea"
weight: 9
toc: false
draft: false
menu:
  sidebar:
    parent: "advanced"
    name: "Настройка Gitea"
    identifier: "customizing-gitea"
    weight: 9
---

# Настройка Gitea

Настройка Gitea обычно выполняется с использованием папки `CustomPath` - по умолчанию это
папка `custom` из текущего каталога, но может быть иным, если в вашей сборке это задано иначе.
Это центральное место для переопределения параметров конфигурации, шаблонов и т.д. Вы можете
проверить `CustomPath` с помощью `gitea help`. Вы также можете найти путь на вкладке _Конфигурации_
на странице _администрации сайта_. Вы можете переопределить `CustomPath`, установив либо переменную
окружения `GITEA_CUSTOM`, либо используя опцию `--custom-path` в двоичном файле `gitea`.
(Параметр переопределит переменную среды.)

Если Gitea развертывается из двоичного кода, все пути по умолчанию будут относиться
к двоичному файлу Gitea. При установке из дистрибутива эти пути, вероятно, будут
изменены на стандарт файловой системы Linux. Gitea попытается создать необходимые
папки, включая `custom/`. Дистрибутивы могут предоставлять символическую ссылку для
`custom`, используя `/etc/gitea/`.

Настройки приложения можно найти в файле `CustomConf`, который по умолчанию
-`CustomPath/conf/app.ini`, но может быть другим, если ваша сборка установила
это иначе. Опять же `gitea help` позволит вам просмотреть эту переменную, и вы
можете переопределить ее, используя опцию `--config` в двоичном файле `gitea`.

- [Быстрая памятка конфигурации](https://docs.gitea.io/ru-ru/config-cheat-sheet/)
- [Complete List](https://github.com/go-gitea/gitea/blob/master/custom/conf/app.example.ini)

Если папку `CustomPath` не удается найти, несмотря на проверку `gitea help`, проверьте переменную
среды `GITEA_CUSTOM`; это можно использовать для переопределения пути по умолчанию к чему-то ещё.
`GITEA_CUSTOM` может, например, быть установлен сценарием инициализации.

- [Список особых переменных](https://docs.gitea.io/ru-ru/specific-variables/)

**Примечание:** Gitea необходимо выполнить полный перезапуск, чтобы увидеть изменения конфигурации.

## Обслуживание пользовательских общедоступных файлов

Чтобы Gitea обслуживала пользовательские общедоступные файлы (например, страницы
и изображения), используйте папку `custom/public/` как корневой каталог. Будут
использоваться символические ссылки.

Например, к файлу `image.png`, хранящемуся в `custom/public/`, можно получить доступ
по URL-адресу. `http://gitea.domain.tld/image.png`.

## Изменение аватара по умолчанию

Поместите изображение png по следующему пути: `custom/public/img/avatar_default.png`

## Настройка страниц и ресурсов Gitea

Исполняемый файл Gitea содержит все ресурсы, необходимые для запуска: шаблоны, изображения,
таблицы стилей и переводы. Любой из них можно переопределить, поместив замену в соответствующий
путь внутри каталога `custom`. Например, чтобы заменить стандартный `.gitignore`, предусмотренный
для репозиториев C++, мы хотим заменить `options/gitignore/C++`. Для этого замена должна быть
помещена в `custom/options/gitignore/C++` (см. О расположении каталога `custom` в верхней
части этого документа).

Каждую страницу Gitea можно изменить. Динамический контент создаётся с использованием [шаблонов go](https://golang.org/pkg/html/template/),
которые можно изменить, поместив замены под каталогом `custom/templates`.

Для получения любого встроенного файла (включая шаблоны) можно использовать инструмент [`gitea embedded`] ({{< relref "doc/advanced/cmd-embedded.ru-ru.md" >}}). Кроме того, их можно найти в каталоге [`templates`] (https://github.com/go-gitea/gitea/tree/master/templates) исходного кода Gitea (Примечание: ссылка на пример взята из `master` (убедитесь, что используете шаблоны, совместимые с версией, которую вы используете).

Имейте в виду, что любой оператор, содержащийся внутри `{{` и `}}`, является синтаксисом
шаблона Gitea, и его нельзя трогать без полного понимания этих компонентов.

### Настройка стартовой / домашней страницы

Скопируйте [`home.tmpl`](https://github.com/go-gitea/gitea/blob/master/templates/home.tmpl) для вашей версии Gitea из `templates` в `custom/templates`.
Редактируйте как хотите.
Не забудьте перезапустить gitea, чтобы изменения вступили в силу.

### Добавление ссылок и вкладок

Если все, что вам нужно, - это добавить дополнительные ссылки на верхнюю панель навигации или нижний колонтитул или дополнительные вкладки в представление репозитория, вы можете поместить их в `extra_links.tmpl` (ссылки добавляются в панель навигации), `extra_links_footer.tmpl` (ссылки добавлен в левую часть нижнего колонтитула) и `extra_tabs.tmpl` внутри вашего каталога `custom/templates/custom/`.

Например, предположим, что вы находитесь в Германии и должны добавить известную юридически обязательную страницу "Impressum"/about, со списком ответственных за содержание сайта:
просто поместите его в каталог "custom/public/" (например, `custom/public/implum.html`) и поместите ссылку на него в любом `custom/templates/custom/extra_links.tmpl` или `custom/templates/custom/extra_links_footer.tmpl`.

Чтобы соответствовать текущему стилю, ссылка должна иметь имя класса "item", и вы можете использовать `{{AppSubUrl}}` для получения базового URL:
`<a class="item" href="{{AppSubUrl}}/impressum.html">Impressum</a>`

Для получения дополнительной информации см. [Добавление юридических страниц](https://docs.gitea.io/ru-ru/adding-legal-pages).

Таким же образом можно добавлять новые вкладки, помещая их в `extra_tabs.tmpl`.
Точный HTML-код, необходимый для соответствия стилю других вкладок, находится в файле
`templates/repo/header.tmpl`
([источник в GitHub](https://github.com/go-gitea/gitea/blob/master/templates/repo/header.tmpl))

### Другие дополнения к странице

Помимо `extra_links.tmpl` и `extra_tabs.tmpl`, есть другие полезные шаблоны, которые вы можете поместить в свой каталог `custom/templates/custom/`:

- `header.tmpl`, непосредственно перед концом тега `<head>`, куда вы можете, например, добавить собственные файлы CSS.
- `body_outer_pre.tmpl`, сразу после начала `<body>`.
- `body_inner_pre.tmpl`, перед верхней панелью навигации, но уже внутри основного контейнера `<div class="full height">`.
- `body_inner_post.tmpl`, до конца основного контейнера.
- `body_outer_post.tmpl`, перед нижним элементом `<footer>`.
- `footer.tmpl`, прямо перед концом тега `<body>` - хорошее место для дополнительного Javascript.

#### Пример: PlantUML

Вы можете добавить поддержку [PlantUML](https://plantuml.com/) в уценку Gitea, используя сервер PlantUML.
Данные кодируются и отправляются на сервер PlantUML, который генерирует изображение.
На http://www.plantuml.com/plantuml есть онлайн-демонстрационный сервер, но если у вас (или ваших пользователей)
есть конфиденциальные данные, вы можете настроить свой собственный [сервер PlantUML](https://plantuml.com/server) вместо.
Чтобы настроить рендеринг PlantUML, скопируйте файлы javascript с https://gitea.com/davidsvantesson/plantuml-code-highlight
и поместите их в свою папку `custom/public`. Затем добавьте следующее в `custom/footer.tmpl`:

```html
{{if .RequireHighlightJS}}
<script src="https://your-server.com/deflate.js"></script>
<script src="https://your-server.com/encode.js"></script>
<script src="https://your-server.com/plantuml_codeblock_parse.js"></script>
<script>
<!-- Replace call with address to your plantuml server-->
parsePlantumlCodeBlocks("http://www.plantuml..com/plantuml")
</script>
{{end}}
```

Затем вы можете добавить следующие блоки в свой markdown:

    ```plantuml
        Alice -> Bob: Authentication Request
        Bob --> Alice: Authentication Response

        Alice -> Bob: Another authentication Request
        Alice <-- Bob: Another authentication Response
    ```

Скрипт обнаружит теги с `class="language-plantuml"`, но вы можете изменить это, указав второй аргумент для `parsePlantumlCodeBlocks`.

#### Пример: Предварительный просмотр STL

Вы можете отобразить файл STL прямо в Gitea, добавив:
```html
<script>
function lS(src){
  return new Promise(function(resolve, reject) {
    let s = document.createElement('script')
    s.src = src
    s.addEventListener('load', () => {
      resolve()
    })
    document.body.appendChild(s)
  });
}

if($('.view-raw>a[href$=".stl" i]').length){
  $('body').append('<link href="/Madeleine.js/src/css/Madeleine.css" rel="stylesheet">');
  Promise.all([lS("/Madeleine.js/src/lib/stats.js"),lS("/Madeleine.js/src/lib/detector.js"), lS("/Madeleine.js/src/lib/three.min.js"), lS("/Madeleine.js/src/Madeleine.js")]).then(function() {
    $('.view-raw').attr('id', 'view-raw').attr('style', 'padding: 0;margin-bottom: -10px;');
    new Madeleine({
      target: 'view-raw',
      data: $('.view-raw>a[href$=".stl" i]').attr('href'),
      path: '/Madeleine.js/src'
    });
    $('.view-raw>a[href$=".stl"]').remove()
  });
}
</script>
```
в файл `templates/custom/footer.tmpl`

Вам также необходимо загрузить содержимое библиотеки [Madeleine.js](https://jinjunho.github.io/Madeleine.js/) и поместить его в папку `custom/public/`.

У вас должна получиться структура папок, похожая на:
```
custom/templates
-- custom
    `-- footer.tmpl
custom/public
-- Madeleine.js
   |-- LICENSE
   |-- README.md
   |-- css
   |   |-- pygment_trac.css
   |   `-- stylesheet.css
   |-- examples
   |   |-- ajax.html
   |   |-- index.html
   |   `-- upload.html
   |-- images
   |   |-- bg_hr.png
   |   |-- blacktocat.png
   |   |-- icon_download.png
   |   `-- sprite_download.png
   |-- models
   |   |-- dino2.stl
   |   |-- ducati.stl
   |   |-- gallardo.stl
   |   |-- lamp.stl
   |   |-- octocat.stl
   |   |-- skull.stl
   |   `-- treefrog.stl
   `-- src
       |-- Madeleine.js
       |-- css
       |   `-- Madeleine.css
       |-- icons
       |   |-- logo.png
       |   |-- madeleine.eot
       |   |-- madeleine.svg
       |   |-- madeleine.ttf
       |   `-- madeleine.woff
       `-- lib
           |-- MadeleineConverter.js
           |-- MadeleineLoader.js
           |-- detector.js
           |-- stats.js
           `-- three.min.js
```

Затем перезапустите gitea и откройте файл STL в своем экземпляре gitea.

## Настройка писем Gitea

Папка `custom/templates/mail` позволяет изменять тело каждого письма Gitea.
Шаблоны для переопределения можно найти в каталоге
[`templates/mail`](https://github.com/go-gitea/gitea/tree/master/templates/mail)
исходного кода Gitea..
Замените, сделав копию файла в папке `custom/templates/mail`, используя
полную структуру пути, соответствующую источнику.

Любой оператор, содержащийся внутри `{{` и `}}`, является синтаксисом
шаблона Gitea, и его нельзя трогать без полного понимания этих компонентов.

## Добавление аналитики в Gitea

Google Analytics, Matomo (ранее Piwik), и другие аналитические сервисы могут быть добавлены в Gitea. Чтобы добавить код отслеживания, обратитесь к разделу `Другие добавления на страницу` этого документа и добавьте код JavaScript в файл `custom/templates/custom/header.tmpl`.

## Настройка gitignores, меток, лицензий, локализации и файлов readme.

Поместите пользовательские файлы в соответствующую подпапку под `custom/options`.

**ПРИМЕЧАНИЕ:** Файлы не должны иметь расширения, например `Labels`, а не `Labels.txt`

### gitignores

Чтобы добавить собственный .gitignore, добавьте файл с существующими [правилами .gitignore](https://git-scm.com/docs/gitignore) в нём в `custom/options/gitignore`

### Метки

Чтобы добавить настраиваемый набор меток, добавьте файл, соответствующий [формату меток](https://github.com/go-gitea/gitea/blob/master/options/label/Default) в `custom/options/label`  
`#hex-color label name ; label description`

### Лицензии

Чтобы добавить пользовательскую лицензию, добавьте файл с текстом лицензии в `custom/options/license`

### Локализации

Управление локализациями осуществляется через наш [crowdin](https://crowdin.com/project/gitea).  
Вы можете переопределить языковой стандарт, поместив изменённый файл языкового стандарта в `custom/options/locale`.  
Файлы локализации Gitea по умолчанию можно найти в исходной папке [`options/locale`](https://github.com/go-gitea/gitea/tree/master/options/locale), и их следует использовать в качестве примеров для ваших изменений.  

Чтобы добавить полностью новый языковой стандарт, а также поместить файл в указанное выше место, вам нужно будет добавить новый язык и имя в раздел `[i18n]` в вашем `app.ini`. Имейте в виду, что Gitea будет использовать эти настройки как **переопределения**, поэтому, если вы хотите сохранить и другие языки, вам нужно будет скопировать/вставить значения по умолчанию и добавить к ним свои собственные.

```
[i18n]
LANGS = en-US,foo-BAR
NAMES = English,FooBar
```

Языки могут меняться от версии к версии, поэтому настоятельно рекомендуется отслеживать свои индивидуальные языковые стандарты.

### Readmes

Чтобы добавить собственный файл Readme, добавьте файл в формате markdown (без расширения `.md`) в `custom/options/readme`

**ПРИМЕЧАНИЕ:** Шаблоны readme поддерживают **расширение переменных**.  
в настоящее время есть `{Name}` (имя репозитория), `{Description}`, `{CloneURL.SSH}`, `{CloneURL.HTTPS}` и `{OwnerName}`

### Реакции

Чтобы изменить эмодзи реакции, вы можете установить разрешённые реакции на app.ini
```
[ui]
REACTIONS = +1, -1, laugh, confused, heart, hooray, eyes
```
Полный список поддерживаемых смайлов находится в [списке смайлов](https://gitea.com/gitea/gitea.com/issues/8)

## Настройка внешнего вида Gitea

Начиная с версии 1.6.0 Gitea имеет встроенные темы. Две встроенные темы - это тема по умолчанию `gitea` и тёмная тема `arc-green`. Чтобы изменить внешний вид вашей установки Gitea, измените значение `DEFAULT_THEME` в разделе [Пользовательский интерфейс](https://docs.gitea.io/ru-ru/config-cheat-sheet/#ui-ui) файла `app.ini` к другому из доступных вариантов.  
Начиная с версии 1.8.0 Gitea также имеет темы для каждого пользователя. Список тем, из которых может выбрать пользователь, можно настроить с помощью значения `THEMES` в разделе [Пользовательский интерфейс](https://docs.gitea.io/ru-ru/config-cheat-sheet/#ui-ui). Из `app.ini` (по умолчанию `gitea` и `arc-green`, светлый и тёмный соответственно)
