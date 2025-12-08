**Zettel Presenter** generates slide shows, handouts, and more from zettels stored in a [Zettelstore](https://zettelstore.de).

## Build Instructions

To build *Zettel Presenter*, simply navigate to the directory of this sub-project and run the following command:

    go build .

This will create an executable named `presenter`.

You can also download the executables for Windows and macOS from <https://zettelstore.de/contrib/uv>.

> **Note**: Please ensure that the version of *Zettel Presenter* matches the version of Zettelstore to avoid compatibility issues.

## Run instructions
    # presenter -h
    Usage of presenter:
    -l string
            Listen address (default: ":23120")
    [URL] URL of Zettelstore (default: "http://127.0.0.1:23123")

* `URL`: Specifies the base URL of the Zettelstore, where the slide zettels are stored.
* `-l`: Defines the listen address, enabling the Zettel Presenter to connect to your browser. If you use the default value, point your browser to <http://127.0.0.1:23120>.

## Configuration

Additional configuration is stored in the metadata of a zettel with the special identifier [00009000001000](https://zettelstore.de/manual/h/00001006055000).
Currently, two keys are supported:

* **`slideset-role`**: Specifies the [zettel role](https://zettelstore.de/manual/h/00001006020100) required for a zettel to be recognized as the starting point of a slide set. The default value is "slideset".
* **`author`**: Defines the default author value for slide shows. By default, it is an empty string, which omits any author information.

## Slide Set

A slide set is a zettel marked with a zettel role defined by the configuration key `slideset-role` (default: "slideset", as mentioned above).
Its primary purpose is to list all zettel that will act as slides for a slideshow.
In other words, it serves as a table of contents.

Internally, the Zettel Presenter looks for the first list in the slide set zettel, whether ordered or unordered.
For each first-level list item, it checks the very first [link reference](https://zettelstore.de/manual/h/00001007040310).
If the link points to a zettel, that zettel will be included in the slide set.

It is perfectly fine to reference the same zettel multiple times, as long as each reference appears in a different first-level item of the slide set zettel.

The second purpose of the slide set zettel is to define metadata needed for the slideshow or handout.
This metadata is stored within the zettel’s metadata and includes:

* **`slide-title`**: Specifies the title of the presentation. By default, it takes the value of the zettel’s title, but you can customize it for the presentation.
* **`sub-title`**: Denotes a subtitle for the presentation. If not specified, no subtitle will be included in the presentation or handout. Like the `slide-title`, you can use Zettelmarkup’s [inline-structured elements](https://zettelstore.de/manual/h/00001007040000) for text formatting.
* **`author`**: Defines the author of the slide set, defaulting to the value specified in the configuration zettel (see above).
* **`copyright`**: Specifies a copyright statement. If not provided, Zettelstore will include a [default copyright statement](https://zettelstore.de/manual/h/00001004020000#default-copyright).
* **`license`**: Allows you to specify a license text. If not provided, Zettelstore will apply a [default license](https://zettelstore.de/manual/h/00001004020000#default-license).

## Slide

A slide is simply a zettel referenced by a slide set zettel.
The Zettel Presenter does not require a specific zettel role for slides.
However, it’s considered good practice to assign a dedicated zettel role, such as "slide".
This makes it easier to find and list specific slides based on their zettel role.

Similar to the slide set zettel, the Zettel Presenter also looks at the metadata of a slide zettel:

* **`slide-title`**: Allows you to override the title of the zettel for the purpose of the presentation.
* **`slide-role`**: Marks a slide zettel to be included only in specific types of presentations: either a slideshow (value: "show") or a handout (value: "handout"). If no value is provided, the slide will be included in all types of presentations. If another value is used, the slide will not appear in any presentation document.

## Slide Roles

Currently, two slide roles are implemented: **slide show** and **handout**.

Presenting a slide show is the primary use case for the Zettel Presenter.
All relevant slides are gathered and an HTML-based slide show is generated.

The handout is another HTML document that contains all relevant slides, but without the interactive slide show elements.
Instead, the slides are presented in a linear format.
Zettel that are referenced but not part of the slide set, yet have the [visibility](https://zettelstore.de/manual/h/00001010070200) set to "public", will be added at the end of the handout for further reference.
This ensures you can provide a complete document to your audience without risking the inclusion of confidential material.

When you reference a zettel from the same slide set, an appropriate HTML link will be created.
Since a zettel might appear more than once in the slide set, the Zettel Presenter searches for references in reverse order (backwards).

If you reference a zettel outside the slide set, it will be linked in the slide show.
Following the link will display the referenced zettel, but not within the context of the slide show.
As mentioned earlier, such links will only appear in the handout if the referenced zettel has "public" visibility.
In this case, it will be considered part of the slide set.

## Navigating

Zettel Presenter operates in a simple, intuitive way, using the same zettel identifiers as Zettelstore.
If no zettel identifier is provided in the URL, Zettel Presenter will display the [home zettel](https://zettelstore.de/manual/h/00001004020000#home-zettel) of Zettelstore.

When the zettel is a slide set, all relevant zettels are collected and used to create a slide show or handout.
These zettels are presented in a numbered or ordered list.
Clicking on any item in the list will take you to the corresponding slide in the slide show.

At the bottom of the slide set, there is a link to generate the handout.

If the zettel is not part of a slide set, it will be displayed in a straightforward manner, similar to how it appears in the Zettelstore web interface.
This view allows you to display additional content (if linked from a slide) or navigate to a slide set zettel to begin a presentation.
