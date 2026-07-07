import * as React from 'react';
import { useHistory } from 'react-router-dom';
import {
  ActionGroup,
  Alert,
  Breadcrumb,
  BreadcrumbItem,
  Button,
  Form,
  FormGroup,
  PageSection,
  TextInput,
  TextInput,
  TextInput,
  TextInput,
  Title,
} from '@patternfly/react-core';
import { createAPIClient } from '../utils/api';

const ProjectCreatePage: React.FC = () => {
  const history = useHistory();
  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const [description, setDescription] = React.useState('');
  const [name, setName] = React.useState('');
  const [repositoryUrl, setRepositoryUrl] = React.useState('');
  const [status, setStatus] = React.useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    const body: Record<string, unknown> = {};
    if (description) body['description'] = description;
    if (name) body['name'] = name;
    if (repositoryUrl) body['repository_url'] = repositoryUrl;
    if (status) body['status'] = status;

    try {
      const api = createAPIClient();
      const created = await api.projects.create(body);
      history.push(`/trex-console/projects/${created.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/projects')}
            component="button"
          >
            Projects
          </BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Create Project</Title>
      </PageSection>
      <PageSection>
        {error && <Alert variant="danger" title="Error" isInline>{error}</Alert>}
        <Form onSubmit={handleSubmit} style={{ maxWidth: 600 }}>
          <FormGroup
            label="Description"
            fieldId="field-description"
            isRequired={ false}
          >
            <TextInput
              id="field-description"
              type="text"
              value={ description}
              onChange={(_e, val) => setDescription(val)}
              placeholder="Enter Description"
              isRequired={ false}
            />
          </FormGroup>
          <FormGroup
            label="Name"
            fieldId="field-name"
            isRequired={ true}
          >
            <TextInput
              id="field-name"
              type="text"
              value={ name}
              onChange={(_e, val) => setName(val)}
              placeholder="Enter Name"
              isRequired={ true}
            />
          </FormGroup>
          <FormGroup
            label="Repository Url"
            fieldId="field-repository_url"
            isRequired={ false}
          >
            <TextInput
              id="field-repository_url"
              type="text"
              value={ repositoryUrl}
              onChange={(_e, val) => setRepositoryUrl(val)}
              placeholder="Enter Repository Url"
              isRequired={ false}
            />
          </FormGroup>
          <FormGroup
            label="Status"
            fieldId="field-status"
            isRequired={ true}
          >
            <TextInput
              id="field-status"
              type="text"
              value={ status}
              onChange={(_e, val) => setStatus(val)}
              placeholder="Enter Status"
              isRequired={ true}
            />
          </FormGroup>
          <ActionGroup>
            <Button type="submit" variant="primary" isLoading={submitting} isDisabled={submitting}>
              Create
            </Button>
            <Button
              variant="link"
              onClick={() => history.push('/trex-console/projects')}
            >
              Cancel
            </Button>
          </ActionGroup>
        </Form>
      </PageSection>
    </>
  );
};

export default ProjectCreatePage;
