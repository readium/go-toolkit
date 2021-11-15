/* global m */

/**
 * Readium navigator demo
 * Henry <chocolatkey@gmail.com>
 * 2021-11-14
 */

let list = undefined;
const Index = {
    oninit: () => {
        fetch("/list.json").then(res => res.json()).then(data => {
            list = data;
            m.redraw();
        });
    },
    view: () => {
        if(list === undefined)
            return m("div", "Loading file list...");
        return [
            m("h1", "Navigator demo for go-toolkit"),
            m("ul", list.map(item => m("li", m(m.route.Link, {href: "/read/" + item.path}, item.filename))))
        ];
    }
}

let manifest = undefined;
let index = 0;
const Reader = {
    oninit: props => {
        manifest = undefined;
        fetch(`/${props.attrs.id}/manifest.json`).then(res => res.json()).then(data => {
            manifest = data;
            index = 0;
            m.redraw();
        })
    },
    view: props => {
        if(manifest === undefined)
            return m("div", "Loading manifest...");

        const item = manifest.readingOrder[index];
        return [
            m("div.flex.three", [
                m(m.route.Link, {href: "/"}, "⇚Back to index"),
                m("select", {onchange: (e) => {
                    index = manifest.readingOrder.findIndex(item => item.href === e.target[e.target.selectedIndex].value);
                }}, manifest.toc?.map(item => m("option", {value: item.href}, item.title))),
                m("", {style: "text-align: right;"}, `${manifest.metadata.title}`)
            ]),
            m("div.flex", m("iframe", {
                style: "height: 85vh",
                src: `/${props.attrs.id}/${item.href}`
            })),
            m("div.flex.three", [
                m("button", {
                    disabled: index === 0,
                    onclick: () => {
                        index--;
                    }
                }, "←Prev"),
                m("pre", {style: "text-align: center;"}, `${index+1}/${manifest.readingOrder.length}`),
                m("button", {
                    disabled: index === manifest.readingOrder.length-1,
                    onclick: () => {
                        index++;
                    }
                }, "Next→")
            ])
        ];
    }
}

// eslint-disable-next-line no-unused-vars
function start() {
    m.route(document.getElementById("root"), "/", {
        "/": Index,
        "/read/:id": Reader
    })
}